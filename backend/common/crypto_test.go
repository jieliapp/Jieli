package common_test

import (
	"strings"
	"testing"

	"github.com/jieliapp/jieli/backend/common"
)

func TestNewAPIKeyShouldReturnPlaintextPrefixAndHashWithoutStoringPlaintext(t *testing.T) {
	credential, err := common.NewAPIKey()
	if err != nil {
		t.Fatalf("new api key: %v", err)
	}

	if !strings.HasPrefix(credential.Plaintext, "jieli_") {
		t.Fatalf("expected plaintext key to use jieli_ prefix, got %q", credential.Plaintext)
	}
	if credential.Prefix == "" || !strings.HasPrefix(credential.Plaintext, credential.Prefix) {
		t.Fatalf("expected public prefix to match plaintext start, got prefix %q plaintext %q", credential.Prefix, credential.Plaintext)
	}
	if credential.Hash == "" {
		t.Fatalf("expected hash to be stored")
	}
	if strings.Contains(credential.Hash, credential.Plaintext) {
		t.Fatalf("expected hash not to contain plaintext key")
	}
	if !common.VerifyAPIKey(credential.Plaintext, credential.Hash) {
		t.Fatalf("expected generated plaintext to verify against hash")
	}
	if common.VerifyAPIKey(credential.Plaintext+"wrong", credential.Hash) {
		t.Fatalf("expected modified key to be rejected")
	}
}

func TestEncryptSecretShouldRoundTripWithSameSecret(t *testing.T) {
	plaintext := "jieli_super_secret_value"
	encrypted, err := common.EncryptSecret(plaintext, "encryption-secret")
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	if encrypted == "" || encrypted == plaintext {
		t.Fatalf("expected ciphertext to differ from plaintext, got %q", encrypted)
	}

	decrypted, err := common.DecryptSecret(encrypted, "encryption-secret")
	if err != nil {
		t.Fatalf("decrypt secret: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("expected round-trip plaintext %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptSecretShouldUseRandomNonce(t *testing.T) {
	first, err := common.EncryptSecret("same-input", "secret")
	if err != nil {
		t.Fatalf("encrypt first: %v", err)
	}
	second, err := common.EncryptSecret("same-input", "secret")
	if err != nil {
		t.Fatalf("encrypt second: %v", err)
	}
	if first == second {
		t.Fatalf("expected distinct ciphertexts for repeated encryption")
	}
}

func TestDecryptSecretShouldFailWithWrongSecret(t *testing.T) {
	encrypted, err := common.EncryptSecret("payload", "right-secret")
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	if _, err := common.DecryptSecret(encrypted, "wrong-secret"); err == nil {
		t.Fatalf("expected decryption with wrong secret to fail")
	}
}
