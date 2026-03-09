// Package secrets provides secret management for Gecko.
// Secrets can be stored locally (encrypted), in Vault, or sourced from env vars.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SecretBackend defines how secrets are stored and retrieved
type SecretBackend interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	List() ([]string, error)
}

// SecretEntry represents a stored secret
type SecretEntry struct {
	Key       string `json:"key"`
	Encrypted string `json:"encrypted"`
	Backend   string `json:"backend"`
}

// LocalEncryptedBackend stores secrets in an encrypted local file
// using AES-256-GCM with a key derived from a passphrase.
type LocalEncryptedBackend struct {
	storePath  string
	key        []byte
}

// NewLocalBackend creates or opens a local secret store
func NewLocalBackend(storePath, passphrase string) (*LocalEncryptedBackend, error) {
	if err := os.MkdirAll(filepath.Dir(storePath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create secret store directory: %w", err)
	}
	// Derive a 32-byte AES key from the passphrase using SHA-256
	hash := sha256.Sum256([]byte(passphrase))
	return &LocalEncryptedBackend{
		storePath: storePath,
		key:       hash[:],
	}, nil
}

// DefaultLocalBackend returns a backend at ~/.gecko/secrets.gecko
func DefaultLocalBackend(passphrase string) (*LocalEncryptedBackend, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".gecko", "secrets.gecko")
	return NewLocalBackend(path, passphrase)
}

func (b *LocalEncryptedBackend) load() (map[string]string, error) {
	data, err := os.ReadFile(b.storePath)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, err
	}
	decrypted, err := b.decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret store (wrong passphrase?): %w", err)
	}
	var store map[string]string
	if err := json.Unmarshal(decrypted, &store); err != nil {
		return nil, err
	}
	return store, nil
}

func (b *LocalEncryptedBackend) save(store map[string]string) error {
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	encrypted, err := b.encrypt(data)
	if err != nil {
		return err
	}
	return os.WriteFile(b.storePath, encrypted, 0600)
}

func (b *LocalEncryptedBackend) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(sealed)))
	base64.StdEncoding.Encode(encoded, sealed)
	return encoded, nil
}

func (b *LocalEncryptedBackend) decrypt(ciphertext []byte) ([]byte, error) {
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(ciphertext)))
	n, err := base64.StdEncoding.Decode(decoded, ciphertext)
	if err != nil {
		return nil, err
	}
	decoded = decoded[:n]

	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(decoded) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, sealed := decoded[:nonceSize], decoded[nonceSize:]
	return gcm.Open(nil, nonce, sealed, nil)
}

// Get retrieves a secret by key
func (b *LocalEncryptedBackend) Get(key string) (string, error) {
	store, err := b.load()
	if err != nil {
		return "", err
	}
	val, ok := store[key]
	if !ok {
		// Fall back to environment variable
		envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
		if envVal := os.Getenv("GECKO_SECRET_" + envKey); envVal != "" {
			return envVal, nil
		}
		return "", fmt.Errorf("secret %q not found", key)
	}
	return val, nil
}

// Set stores a secret
func (b *LocalEncryptedBackend) Set(key, value string) error {
	store, err := b.load()
	if err != nil {
		return err
	}
	store[key] = value
	return b.save(store)
}

// Delete removes a secret
func (b *LocalEncryptedBackend) Delete(key string) error {
	store, err := b.load()
	if err != nil {
		return err
	}
	delete(store, key)
	return b.save(store)
}

// List returns all secret keys (not values)
func (b *LocalEncryptedBackend) List() ([]string, error) {
	store, err := b.load()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(store))
	for k := range store {
		keys = append(keys, k)
	}
	return keys, nil
}

// EnvBackend reads secrets from environment variables only
type EnvBackend struct{}

func (e *EnvBackend) Get(key string) (string, error) {
	envKey := "GECKO_SECRET_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
	val := os.Getenv(envKey)
	if val == "" {
		return "", fmt.Errorf("env var %s not set", envKey)
	}
	return val, nil
}

func (e *EnvBackend) Set(_, _ string) error {
	return fmt.Errorf("env backend is read-only")
}
func (e *EnvBackend) Delete(_ string) error { return fmt.Errorf("env backend is read-only") }
func (e *EnvBackend) List() ([]string, error) {
	var keys []string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "GECKO_SECRET_") {
			parts := strings.SplitN(env, "=", 2)
			keys = append(keys, strings.ToLower(strings.TrimPrefix(parts[0], "GECKO_SECRET_")))
		}
	}
	return keys, nil
}
