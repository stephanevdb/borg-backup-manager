package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/fernet/fernet-go"
)

type Encryptor struct {
	key *fernet.Key
}

func NewEncryptor(secretKey string) (*Encryptor, error) {
	if secretKey == "" {
		return nil, fmt.Errorf("secret key is required")
	}
	h := sha256.Sum256([]byte(secretKey))
	keyStr := base64.URLEncoding.EncodeToString(h[:])
	key, err := fernet.DecodeKey(keyStr)
	if err != nil {
		return nil, err
	}
	return &Encryptor{key: key}, nil
}

func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	tok, err := fernet.EncryptAndSign([]byte(plaintext), e.key)
	if err != nil {
		return "", err
	}
	return string(tok), nil
}

func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	plain := fernet.VerifyAndDecrypt([]byte(ciphertext), 0, []*fernet.Key{e.key})
	if plain == nil {
		return "", fmt.Errorf("decryption failed")
	}
	return string(plain), nil
}
