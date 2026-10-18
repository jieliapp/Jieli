package attachment

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jieliapp/jieli/backend/model"
)

const MaxUploadBytes = 10 * 1024 * 1024

const localStorageDriver = "local"

var storage = newStorageRegistry(NewLocalStorage("data/attachments"))

var (
	ErrUserRequired         = errors.New("user is required")
	ErrInvalidPayload       = errors.New("invalid attachment payload")
	ErrUnsupportedMediaType = errors.New("unsupported media type")
	ErrInvalidBase64        = errors.New("invalid base64 data")
	ErrMediaTypeMismatch    = errors.New("media type does not match data")
	ErrTooLarge             = errors.New("attachment is too large")
	ErrNotFound             = errors.New("attachment not found")
)

type StorageDriver interface {
	DriverName() string
	Write(key string, data []byte) error
	Read(key string) ([]byte, error)
	Delete(key string) error
}

type storageRegistry struct {
	mutex   sync.RWMutex
	active  StorageDriver
	drivers map[string]StorageDriver
}

func newStorageRegistry(active StorageDriver, fallbackDrivers ...StorageDriver) *storageRegistry {
	registry := &storageRegistry{drivers: map[string]StorageDriver{}}
	registry.configure(active, fallbackDrivers...)
	return registry
}

func (registry *storageRegistry) configure(active StorageDriver, fallbackDrivers ...StorageDriver) {
	registry.mutex.Lock()
	defer registry.mutex.Unlock()
	registry.active = active
	registry.drivers = map[string]StorageDriver{}
	if active != nil {
		registry.drivers[active.DriverName()] = active
	}
	for _, driver := range fallbackDrivers {
		if driver != nil {
			registry.drivers[driver.DriverName()] = driver
		}
	}
}

func (registry *storageRegistry) writeDriver() StorageDriver {
	registry.mutex.RLock()
	defer registry.mutex.RUnlock()
	return registry.active
}

func (registry *storageRegistry) driver(name string) (StorageDriver, bool) {
	registry.mutex.RLock()
	defer registry.mutex.RUnlock()
	driver, ok := registry.drivers[name]
	return driver, ok
}

type LocalStorage struct {
	RootDir string
}

func NewLocalStorage(rootDir string) LocalStorage {
	return LocalStorage{RootDir: rootDir}
}

func ConfigureStorage(driver StorageDriver, fallbackDrivers ...StorageDriver) {
	storage.configure(driver, fallbackDrivers...)
}

func (store LocalStorage) DriverName() string {
	return localStorageDriver
}

func (store LocalStorage) Write(key string, data []byte) error {
	path, err := store.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (store LocalStorage) Read(key string) ([]byte, error) {
	path, err := store.pathForKey(key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (store LocalStorage) Delete(key string) error {
	path, err := store.pathForKey(key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (store LocalStorage) pathForKey(key string) (string, error) {
	if key == "" || filepath.IsAbs(key) {
		return "", ErrNotFound
	}
	root, err := filepath.Abs(store.RootDir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.Clean(key))
	if path != root && !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return "", ErrNotFound
	}
	return path, nil
}

type PublicAttachment struct {
	Attachment *model.Attachment
	Data       []byte
	MediaType  string
}

type UploadPayload struct {
	Data      string `json:"data"`
	MediaType string `json:"mediaType"`
}

func Upload(user *model.User, payload UploadPayload) (*model.Attachment, error) {
	if user == nil || user.ID == 0 {
		return nil, ErrUserRequired
	}
	extension, ok := extensionForMediaType(payload.MediaType)
	if !ok {
		return nil, ErrUnsupportedMediaType
	}
	decoded, err := decodeBase64(payload.Data)
	if err != nil {
		return nil, err
	}
	if len(decoded) == 0 {
		return nil, ErrInvalidPayload
	}
	if len(decoded) > MaxUploadBytes {
		return nil, ErrTooLarge
	}
	if !matchesMediaType(decoded, payload.MediaType) {
		return nil, ErrMediaTypeMismatch
	}
	publicID := newPublicID()
	storageKey := publicID + "." + extension
	driver := storage.writeDriver()
	if driver == nil {
		return nil, ErrNotFound
	}
	if err := driver.Write(storageKey, decoded); err != nil {
		return nil, err
	}
	stored := &model.Attachment{
		UserID:        user.ID,
		PublicID:      publicID,
		MediaType:     payload.MediaType,
		Extension:     extension,
		StorageDriver: driver.DriverName(),
		StorageKey:    storageKey,
		SizeBytes:     len(decoded),
	}
	if err := model.AttachmentDB.Save(stored); err != nil {
		_ = driver.Delete(storageKey)
		return nil, err
	}
	return stored, nil
}

func GetPublic(idOrFilename string) (*PublicAttachment, error) {
	publicID, extension := splitPublicID(idOrFilename)
	if publicID == "" {
		return nil, ErrNotFound
	}
	attachments, err := model.AttachmentDB.Where("public_id = ?", publicID).Fetch(0, 1)
	if err != nil {
		return nil, err
	}
	if len(attachments) != 1 {
		return nil, ErrNotFound
	}
	stored := attachments[0]
	if extension != "" && extension != stored.Extension {
		return nil, ErrNotFound
	}
	if stored.StorageKey == "" {
		return nil, ErrNotFound
	}
	driver, ok := storage.driver(stored.StorageDriver)
	if !ok {
		return nil, ErrNotFound
	}
	data, err := driver.Read(stored.StorageKey)
	if err != nil {
		return nil, ErrNotFound
	}
	return &PublicAttachment{Attachment: stored, Data: data, MediaType: stored.MediaType}, nil
}

func extensionForMediaType(mediaType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/png":
		return "png", true
	case "image/jpeg":
		return "jpg", true
	case "image/gif":
		return "gif", true
	case "image/webp":
		return "webp", true
	default:
		return "", false
	}
}

func decodeBase64(data string) ([]byte, error) {
	trimmed := strings.TrimSpace(data)
	if comma := strings.Index(trimmed, ","); strings.HasPrefix(trimmed, "data:") && comma >= 0 {
		trimmed = trimmed[comma+1:]
	}
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err == nil {
		return decoded, nil
	}
	decoded, err = base64.RawStdEncoding.DecodeString(trimmed)
	if err == nil {
		return decoded, nil
	}
	decoded, err = base64.URLEncoding.DecodeString(trimmed)
	if err == nil {
		return decoded, nil
	}
	decoded, err = base64.RawURLEncoding.DecodeString(trimmed)
	if err == nil {
		return decoded, nil
	}
	return nil, ErrInvalidBase64
}

func matchesMediaType(data []byte, mediaType string) bool {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/png":
		return hasPrefix(data, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	case "image/jpeg":
		return hasPrefix(data, []byte{0xff, 0xd8, 0xff})
	case "image/gif":
		return hasPrefix(data, []byte("GIF87a")) || hasPrefix(data, []byte("GIF89a"))
	case "image/webp":
		return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	default:
		return false
	}
}

func hasPrefix(data []byte, prefix []byte) bool {
	return len(data) >= len(prefix) && string(data[:len(prefix)]) == string(prefix)
}

func splitPublicID(idOrFilename string) (string, string) {
	trimmed := strings.TrimSpace(idOrFilename)
	if trimmed == "" {
		return "", ""
	}
	index := strings.LastIndex(trimmed, ".")
	if index <= 0 || index == len(trimmed)-1 {
		return trimmed, ""
	}
	return trimmed[:index], trimmed[index+1:]
}

func newPublicID() string {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("attachment-%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buffer)
}
