package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	apiKeyPrefix      = "jieli_"
	apiKeyRandomBytes = 32
	apiKeyPublicChars = 14
)

type APIKeyCredential struct {
	Plaintext string
	Prefix    string
	Hash      string
}

func NewAPIKey() (APIKeyCredential, error) {
	random := make([]byte, apiKeyRandomBytes)
	if _, err := rand.Read(random); err != nil {
		return APIKeyCredential{}, err
	}

	plaintext := apiKeyPrefix + strings.TrimRight(base64.URLEncoding.EncodeToString(random), "=")
	hash, err := HashAPIKey(plaintext)
	if err != nil {
		return APIKeyCredential{}, err
	}
	return APIKeyCredential{
		Plaintext: plaintext,
		Prefix:    plaintext[:apiKeyPublicChars],
		Hash:      hash,
	}, nil
}

func HashAPIKey(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	return string(hash), err
}

func VerifyAPIKey(plaintext string, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}

func APIKeyPublicPrefix(plaintext string) string {
	if len(plaintext) < apiKeyPublicChars || !strings.HasPrefix(plaintext, apiKeyPrefix) {
		return ""
	}
	return plaintext[:apiKeyPublicChars]
}

// EncryptSecret encrypts plaintext with AES-256-GCM using a key derived from
// secret via SHA-256, so any-length secret (including a default) is accepted.
// The output is base64(nonce || ciphertext).
func EncryptSecret(plaintext string, secret string) (string, error) {
	gcm, err := newSecretGCM(secret)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptSecret reverses EncryptSecret using the same secret.
func DecryptSecret(encoded string, secret string) (string, error) {
	gcm, err := newSecretGCM(secret)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func newSecretGCM(secret string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
