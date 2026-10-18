package attachment_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/attachment"
)

var tinyPNG = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00}

func TestUploadShouldStoreImageAttachmentForUser(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	storageDir := t.TempDir()
	attachment.ConfigureStorage(attachment.NewLocalStorage(storageDir))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	stored, err := attachment.Upload(user, attachment.UploadPayload{
		Data:      base64.StdEncoding.EncodeToString(tinyPNG),
		MediaType: "image/png",
	})
	if err != nil {
		t.Fatalf("upload attachment: %v", err)
	}
	if stored.ID == 0 || stored.UserID != user.ID || stored.PublicID == "" {
		t.Fatalf("expected saved attachment for user, got %#v", stored)
	}
	if stored.MediaType != "image/png" || stored.Extension != "png" || stored.SizeBytes != len(tinyPNG) || stored.StorageDriver != "local" || stored.StorageKey == "" {
		t.Fatalf("expected png metadata with local storage key, got %#v", stored)
	}
	fileBytes, err := os.ReadFile(filepath.Join(storageDir, stored.StorageKey))
	if err != nil {
		t.Fatalf("read stored attachment file: %v", err)
	}
	if !bytes.Equal(fileBytes, tinyPNG) {
		t.Fatalf("expected attachment file to contain original bytes, got %#v", fileBytes)
	}

	loaded, err := attachment.GetPublic(stored.PublicID + ".png")
	if err != nil {
		t.Fatalf("get public attachment: %v", err)
	}
	if !bytes.Equal(loaded.Data, tinyPNG) || loaded.MediaType != "image/png" {
		t.Fatalf("expected original png bytes and content type, got %#v", loaded)
	}
}

func TestUploadShouldRejectInvalidBase64(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	_, err := attachment.Upload(user, attachment.UploadPayload{Data: "not base64", MediaType: "image/png"})
	if !errors.Is(err, attachment.ErrInvalidBase64) {
		t.Fatalf("expected invalid base64 error, got %v", err)
	}
}

func TestUploadShouldRejectUnsupportedMediaType(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	_, err := attachment.Upload(user, attachment.UploadPayload{
		Data:      base64.StdEncoding.EncodeToString(tinyPNG),
		MediaType: "image/svg+xml",
	})
	if !errors.Is(err, attachment.ErrUnsupportedMediaType) {
		t.Fatalf("expected unsupported media type error, got %v", err)
	}
}

func TestUploadShouldRejectMediaTypeMismatch(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	_, err := attachment.Upload(user, attachment.UploadPayload{
		Data:      base64.StdEncoding.EncodeToString([]byte("not a png")),
		MediaType: "image/png",
	})
	if !errors.Is(err, attachment.ErrMediaTypeMismatch) {
		t.Fatalf("expected media type mismatch error, got %v", err)
	}
}

func TestGetPublicShouldReadAttachmentFromStoredDriverAfterWriteDriverChanges(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	localStorage := newMemoryStorage("local")
	attachment.ConfigureStorage(localStorage)
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	stored, err := attachment.Upload(user, attachment.UploadPayload{
		Data:      base64.StdEncoding.EncodeToString(tinyPNG),
		MediaType: "image/png",
	})
	if err != nil {
		t.Fatalf("upload attachment: %v", err)
	}
	attachment.ConfigureStorage(newMemoryStorage("r2"), localStorage)

	loaded, err := attachment.GetPublic(stored.PublicID + ".png")
	if err != nil {
		t.Fatalf("expected old local attachment to remain readable after switching write driver: %v", err)
	}
	if !bytes.Equal(loaded.Data, tinyPNG) || loaded.MediaType != "image/png" {
		t.Fatalf("expected original local attachment bytes after switching write driver, got %#v", loaded)
	}
}

func closeModelDB(t *testing.T) func() {
	t.Helper()
	return func() {
		if err := model.CloseDB(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}
}

type memoryStorage struct {
	driverName string
	objects    map[string][]byte
}

func newMemoryStorage(driverName string) *memoryStorage {
	return &memoryStorage{driverName: driverName, objects: map[string][]byte{}}
}

func (storage *memoryStorage) DriverName() string {
	return storage.driverName
}

func (storage *memoryStorage) Write(key string, data []byte) error {
	storage.objects[key] = append([]byte(nil), data...)
	return nil
}

func (storage *memoryStorage) Read(key string) ([]byte, error) {
	data, ok := storage.objects[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (storage *memoryStorage) Delete(key string) error {
	delete(storage.objects, key)
	return nil
}
