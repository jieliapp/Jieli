package attachment

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestR2StorageShouldWriteReadAndDeleteObjects(t *testing.T) {
	client := &fakeR2Client{readBody: "stored bytes"}
	store := R2Storage{Bucket: "jieli-attachments", Client: client}

	if err := store.Write("/attachment.png", []byte("png bytes")); err != nil {
		t.Fatalf("write R2 object: %v", err)
	}
	if aws.ToString(client.putInput.Bucket) != "jieli-attachments" || aws.ToString(client.putInput.Key) != "attachment.png" {
		t.Fatalf("expected put bucket/key to target attachment object, got %#v", client.putInput)
	}
	if aws.ToString(client.putInput.ContentType) != "image/png" {
		t.Fatalf("expected png content type, got %q", aws.ToString(client.putInput.ContentType))
	}
	if aws.ToString(client.putInput.CacheControl) != "public, max-age=31536000, immutable" {
		t.Fatalf("expected immutable cache control, got %q", aws.ToString(client.putInput.CacheControl))
	}
	body, err := io.ReadAll(client.putInput.Body)
	if err != nil {
		t.Fatalf("read put body: %v", err)
	}
	if string(body) != "png bytes" {
		t.Fatalf("expected original write bytes, got %q", string(body))
	}

	data, err := store.Read("attachment.png")
	if err != nil {
		t.Fatalf("read R2 object: %v", err)
	}
	if string(data) != "stored bytes" {
		t.Fatalf("expected R2 read bytes, got %q", string(data))
	}
	if aws.ToString(client.getInput.Bucket) != "jieli-attachments" || aws.ToString(client.getInput.Key) != "attachment.png" {
		t.Fatalf("expected get bucket/key to target attachment object, got %#v", client.getInput)
	}

	if err := store.Delete("attachment.png"); err != nil {
		t.Fatalf("delete R2 object: %v", err)
	}
	if aws.ToString(client.deleteInput.Bucket) != "jieli-attachments" || aws.ToString(client.deleteInput.Key) != "attachment.png" {
		t.Fatalf("expected delete bucket/key to target attachment object, got %#v", client.deleteInput)
	}
}

type fakeR2Client struct {
	putInput    *s3.PutObjectInput
	getInput    *s3.GetObjectInput
	deleteInput *s3.DeleteObjectInput
	readBody    string
}

func (client *fakeR2Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	client.putInput = params
	return &s3.PutObjectOutput{}, nil
}

func (client *fakeR2Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	client.getInput = params
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(client.readBody))}, nil
}

func (client *fakeR2Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	client.deleteInput = params
	return &s3.DeleteObjectOutput{}, nil
}
