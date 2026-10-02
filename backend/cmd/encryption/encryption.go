package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Prefix marks values encrypted with key version 1 (AES-256-GCM).
// A future key rotation can introduce "v2:" while still reading "v1:".
const Prefix = "v1:"

const keySize = 32

var (
	ErrNoKey      = errors.New("encryption key not initialized")
	ErrMissingKey = errors.New("ENCRYPTION_KEY environment variable not set")
	ErrInvalidKey = errors.New("ENCRYPTION_KEY must be 32 bytes, base64-encoded")
	ErrDecrypt    = errors.New("failed to decrypt value")
)

var aead cipher.AEAD

// LoadFromEnv initializes the key from ENCRYPTION_KEY, or from the file named
// by ENCRYPTION_KEY_FILE (e.g. a Docker secret) when the former is unset.
func LoadFromEnv() error {
	raw := os.Getenv("ENCRYPTION_KEY")
	if path := os.Getenv("ENCRYPTION_KEY_FILE"); strings.TrimSpace(raw) == "" && path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read ENCRYPTION_KEY_FILE: %w", err)
		}
		raw = string(b)
	}
	if strings.TrimSpace(raw) == "" {
		return ErrMissingKey
	}
	return Init(raw)
}

// Init sets the process-wide key from its base64 encoding.
func Init(keyB64 string) error {
	key, err := decodeKey(strings.TrimSpace(keyB64))
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	aead = g
	return nil
}

func decodeKey(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == keySize {
			return b, nil
		}
	}
	return nil, ErrInvalidKey
}

func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, Prefix)
}

// Encrypt returns Prefix + base64(nonce || ciphertext). Empty input stays empty.
func Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if aead == nil {
		return "", ErrNoKey
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(plain), nil)
	return Prefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. Values without Prefix are legacy plaintext and are
// returned unchanged so rows written before encryption keep working.
func Decrypt(s string) (string, error) {
	if !IsEncrypted(s) {
		return s, nil
	}
	if aead == nil {
		return "", ErrNoKey
	}
	raw, err := base64.RawStdEncoding.DecodeString(s[len(Prefix):])
	n := aead.NonceSize()
	if err != nil || len(raw) < n {
		return "", ErrDecrypt
	}
	plain, err := aead.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", ErrDecrypt
	}
	return string(plain), nil
}

// DecryptInPlace decrypts each value in place, stopping at the first error.
func DecryptInPlace(vals ...*string) error {
	for _, v := range vals {
		p, err := Decrypt(*v)
		if err != nil {
			return err
		}
		*v = p
	}
	return nil
}
