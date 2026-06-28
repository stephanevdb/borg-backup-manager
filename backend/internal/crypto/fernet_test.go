package crypto_test

import (
	"testing"

	"github.com/borg-backup-manager/backend/internal/crypto"
)

func TestEncryptDecrypt(t *testing.T) {
	enc, err := crypto.NewEncryptor("test-secret-key-for-fernet")
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := enc.Encrypt("hello world")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := enc.Decrypt(cipher)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "hello world" {
		t.Fatalf("got %q", plain)
	}
}
