package route_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/route"
	"github.com/jieliapp/jieli/backend/model"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
)

type repeatingByteReader byte

func (reader repeatingByteReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = byte(reader)
	}
	return len(buffer), nil
}

type blockingRequestBody struct {
	reader  *strings.Reader
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingRequestBody(body string) *blockingRequestBody {
	return &blockingRequestBody{
		reader:  strings.NewReader(body),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (body *blockingRequestBody) Read(buffer []byte) (int, error) {
	body.once.Do(func() {
		close(body.started)
		<-body.release
	})
	return body.reader.Read(buffer)
}

func (body *blockingRequestBody) Close() error {
	return nil
}

type trackingRequestBody struct {
	reader *strings.Reader
	read   atomic.Bool
}

func (body *trackingRequestBody) Read(buffer []byte) (int, error) {
	body.read.Store(true)
	return body.reader.Read(buffer)
}

func (body *trackingRequestBody) Close() error {
	return nil
}

func TestThreadUploadRejectsBodyOverLimitBeforeJSONDecode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	credential, err := authservice.ProvisionUser(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})

	body := io.LimitReader(repeatingByteReader(' '), 64*1024*1024+1)
	request := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", body)
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestThreadUploadWaitsForSameUserPermitBeforeReadingJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	credential, err := authservice.ProvisionUser(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})

	firstBody := newBlockingRequestBody(`{"provider":"codex","thread":{"id":"T-first","messages":[]}}`)
	firstRequest := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", firstBody)
	firstRequest.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	firstRecorder := httptest.NewRecorder()
	firstDone := make(chan struct{})
	go func() {
		router.ServeHTTP(firstRecorder, firstRequest)
		close(firstDone)
	}()
	select {
	case <-firstBody.started:
	case <-time.After(2 * time.Second):
		close(firstBody.release)
		t.Fatal("first upload did not start reading its body")
	}

	secondBody := &trackingRequestBody{reader: strings.NewReader(`{"provider":"codex","thread":{"id":"T-second","messages":[]}}`)}
	secondRequest := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", secondBody)
	secondRequest.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	cancelledContext, cancel := context.WithCancel(secondRequest.Context())
	cancel()
	secondRequest = secondRequest.WithContext(cancelledContext)
	secondRecorder := httptest.NewRecorder()
	router.ServeHTTP(secondRecorder, secondRequest)

	if secondRecorder.Code != http.StatusRequestTimeout {
		close(firstBody.release)
		<-firstDone
		t.Fatalf("expected cancelled waiter status 408, got %d body %s", secondRecorder.Code, secondRecorder.Body.String())
	}
	if secondBody.read.Load() {
		close(firstBody.release)
		<-firstDone
		t.Fatal("expected same-user waiter not to read JSON before acquiring a permit")
	}
	close(firstBody.release)
	<-firstDone
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("expected first upload status 200, got %d body %s", firstRecorder.Code, firstRecorder.Body.String())
	}
}

func TestThreadUploadAllowsDifferentUsersToProceedIndependently(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	aliceCredential, err := authservice.ProvisionUser(&model.User{Username: "alice", GitHubID: "github-alice"})
	if err != nil {
		t.Fatalf("rotate alice api key: %v", err)
	}
	bobCredential, err := authservice.ProvisionUser(&model.User{Username: "bob", GitHubID: "github-bob"})
	if err != nil {
		t.Fatalf("rotate bob api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})

	firstBody := newBlockingRequestBody(`{"provider":"codex","thread":{"id":"T-alice","messages":[]}}`)
	firstRequest := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", firstBody)
	firstRequest.Header.Set("Authorization", "Bearer "+aliceCredential.Plaintext)
	firstRecorder := httptest.NewRecorder()
	firstDone := make(chan struct{})
	go func() {
		router.ServeHTTP(firstRecorder, firstRequest)
		close(firstDone)
	}()
	select {
	case <-firstBody.started:
	case <-time.After(2 * time.Second):
		close(firstBody.release)
		t.Fatal("alice upload did not start reading its body")
	}

	secondRequest := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", strings.NewReader(`{"provider":"codex","thread":{"id":"T-bob","messages":[]}}`))
	secondRequest.Header.Set("Authorization", "Bearer "+bobCredential.Plaintext)
	secondRecorder := httptest.NewRecorder()
	secondDone := make(chan struct{})
	go func() {
		router.ServeHTTP(secondRecorder, secondRequest)
		close(secondDone)
	}()
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		close(firstBody.release)
		<-firstDone
		t.Fatal("bob upload was blocked by alice's user-level permit")
	}
	if secondRecorder.Code != http.StatusOK {
		close(firstBody.release)
		<-firstDone
		t.Fatalf("expected bob upload status 200, got %d body %s", secondRecorder.Code, secondRecorder.Body.String())
	}

	close(firstBody.release)
	<-firstDone
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("expected alice upload status 200, got %d body %s", firstRecorder.Code, firstRecorder.Body.String())
	}
}
