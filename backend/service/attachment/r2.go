package attachment

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jieliapp/jieli/backend/common"
)

const r2StorageDriver = "r2"

type R2Storage struct {
	Bucket string
	Client r2Client
}

type r2Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

func NewR2Storage(cfg common.AttachmentStorageConfig) R2Storage {
	region := strings.TrimSpace(cfg.R2Region)
	if region == "" {
		region = common.DefaultR2Region
	}
	awsConfig := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, ""),
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(strings.TrimRight(cfg.R2Endpoint, "/"))
		options.UsePathStyle = true
	})
	return R2Storage{Bucket: cfg.R2Bucket, Client: client}
}

func ConfigureStorageFromConfig(cfg common.AttachmentStorageConfig) error {
	if err := common.ValidateAttachmentStorage(cfg); err != nil {
		return err
	}
	switch strings.TrimSpace(cfg.Driver) {
	case "", common.DefaultAttachmentStorageDriver:
		ConfigureStorage(NewLocalStorage(cfg.LocalDir))
	case common.R2AttachmentStorageDriver:
		ConfigureStorage(NewR2Storage(cfg), NewLocalStorage(cfg.LocalDir))
	}
	return nil
}

func (store R2Storage) DriverName() string {
	return r2StorageDriver
}

func (store R2Storage) Write(key string, data []byte) error {
	_, err := store.Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:       aws.String(store.Bucket),
		Key:          aws.String(cleanObjectKey(key)),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentTypeForObjectKey(key)),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	return err
}

func (store R2Storage) Read(key string) ([]byte, error) {
	result, err := store.Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(store.Bucket),
		Key:    aws.String(cleanObjectKey(key)),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	return io.ReadAll(result.Body)
}

func (store R2Storage) Delete(key string) error {
	_, err := store.Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(store.Bucket),
		Key:    aws.String(cleanObjectKey(key)),
	})
	return err
}

func cleanObjectKey(key string) string {
	return strings.TrimLeft(strings.TrimSpace(key), "/")
}

func contentTypeForObjectKey(key string) string {
	index := strings.LastIndex(key, ".")
	if index < 0 || index == len(key)-1 {
		return "application/octet-stream"
	}
	switch strings.ToLower(strings.TrimSpace(key[index+1:])) {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
