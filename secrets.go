package vcenter

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	encryptionVersion = 1
	saltLength        = 16
	nonceLength       = 12
	derivedKeyLength  = 32
)

// Credential mirrors the Python structure for credential entries.
type Credential struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}

// CredentialStore stores multiple credentials and can serialize plain or encrypted JSON.
type CredentialStore struct {
	Credentials map[string]Credential `json:"credentials"`
}

// NewCredentialStore creates an empty credential store.
func NewCredentialStore() *CredentialStore {
	return &CredentialStore{Credentials: map[string]Credential{}}
}

// AddCredential adds or overwrites a credential.
func (s *CredentialStore) AddCredential(name string, cred Credential) {
	if s.Credentials == nil {
		s.Credentials = map[string]Credential{}
	}
	s.Credentials[name] = cred
}

// GetCredential fetches a credential by name.
func (s *CredentialStore) GetCredential(name string) (Credential, bool) {
	cred, ok := s.Credentials[name]
	return cred, ok
}

// ListNames returns credential names sorted alphabetically.
func (s *CredentialStore) ListNames() []string {
	names := make([]string, 0, len(s.Credentials))
	for name := range s.Credentials {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SavePlaintext writes the credential store as JSON (use only for initial setup).
func (s *CredentialStore) SavePlaintext(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadPlaintextCredentialStore reads credentials without encryption.
func LoadPlaintextCredentialStore(path string) (*CredentialStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	var store CredentialStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if store.Credentials == nil {
		store.Credentials = map[string]Credential{}
	}
	return &store, nil
}

// KeySource represents where to fetch the master secret used for encryption.
type KeySource struct {
	kind  string
	value string
}

// KeySourceEnv uses an environment variable.
func KeySourceEnv(name string) KeySource {
	return KeySource{kind: "env", value: name}
}

// KeySourceFile reads key bytes from a file (trimmed).
func KeySourceFile(path string) KeySource {
	return KeySource{kind: "file", value: path}
}

// KeySourceDirect uses the provided string as the secret.
func KeySourceDirect(secret string) KeySource {
	return KeySource{kind: "direct", value: secret}
}

func (ks KeySource) resolveSecret() ([]byte, error) {
	switch ks.kind {
	case "env":
		val := os.Getenv(ks.value)
		if val == "" {
			return nil, fmt.Errorf("environment variable %s is not set", ks.value)
		}
		return []byte(val), nil
	case "file":
		data, err := os.ReadFile(ks.value)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
		return []byte(strings.TrimSpace(string(data))), nil
	case "direct":
		if ks.value == "" {
			return nil, errors.New("direct key source is empty")
		}
		return []byte(ks.value), nil
	default:
		return nil, fmt.Errorf("unknown key source: %s", ks.kind)
	}
}

func (ks KeySource) deriveKey(salt []byte) ([]byte, error) {
	secret, err := ks.resolveSecret()
	if err != nil {
		return nil, err
	}
	return argon2.IDKey(secret, salt, 2, 64*1024, 1, derivedKeyLength), nil
}

type encryptedEnvelope struct {
	Version    int    `json:"version"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// EncryptString encrypts plaintext using AES-256-GCM with a KeySource-derived key.
func EncryptString(plaintext string, ks KeySource) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	key, err := ks.deriveKey(salt)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, nonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext, err := encryptAESGCM([]byte(plaintext), key, nonce)
	if err != nil {
		return "", err
	}

	env := encryptedEnvelope{
		Version:    encryptionVersion,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	data, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("marshal envelope: %w", err)
	}
	return string(data), nil
}

// DecryptString decrypts text produced by EncryptString.
func DecryptString(encrypted string, ks KeySource) (string, error) {
	var env encryptedEnvelope
	if err := json.Unmarshal([]byte(encrypted), &env); err != nil {
		return "", fmt.Errorf("parse encrypted data: %w", err)
	}
	if env.Version != encryptionVersion {
		return "", fmt.Errorf("unsupported encryption version: %d", env.Version)
	}

	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil {
		return "", fmt.Errorf("decode salt: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	key, err := ks.deriveKey(salt)
	if err != nil {
		return "", err
	}
	plaintext, err := decryptAESGCM(ciphertext, key, nonce)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptStringWithPassword is a simpler variant that derives the key from a master password.
func EncryptStringWithPassword(plaintext string, password string) (string, error) {
	if password == "" {
		return "", errors.New("password is required")
	}
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, 2, 64*1024, 1, derivedKeyLength)

	nonce := make([]byte, nonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext, err := encryptAESGCM([]byte(plaintext), key, nonce)
	if err != nil {
		return "", err
	}

	env := encryptedEnvelope{
		Version:    encryptionVersion,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	data, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("marshal envelope: %w", err)
	}
	return string(data), nil
}

// DecryptStringWithPassword decrypts payload encrypted with EncryptStringWithPassword.
func DecryptStringWithPassword(encrypted string, password string) (string, error) {
	if password == "" {
		return "", errors.New("password is required")
	}
	var env encryptedEnvelope
	if err := json.Unmarshal([]byte(encrypted), &env); err != nil {
		return "", fmt.Errorf("parse encrypted data: %w", err)
	}
	if env.Version != encryptionVersion {
		return "", fmt.Errorf("unsupported encryption version: %d", env.Version)
	}

	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil {
		return "", fmt.Errorf("decode salt: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, 2, 64*1024, 1, derivedKeyLength)
	plaintext, err := decryptAESGCM(ciphertext, key, nonce)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// SaveEncrypted saves credentials encrypted with a KeySource-derived key.
func (s *CredentialStore) SaveEncrypted(path string, ks KeySource) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	encrypted, err := EncryptString(string(data), ks)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(encrypted), 0o600)
}

// LoadEncryptedCredentialStore decrypts and parses credentials using a KeySource.
func LoadEncryptedCredentialStore(path string, ks KeySource) (*CredentialStore, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read encrypted credentials: %w", err)
	}
	decrypted, err := DecryptString(string(payload), ks)
	if err != nil {
		return nil, err
	}
	var store CredentialStore
	if err := json.Unmarshal([]byte(decrypted), &store); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if store.Credentials == nil {
		store.Credentials = map[string]Credential{}
	}
	return &store, nil
}

// SaveEncryptedWithPassword encrypts using a human-friendly master password workflow.
func (s *CredentialStore) SaveEncryptedWithPassword(path string, password string) error {
	if password == "" {
		return errors.New("password is required")
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	encrypted, err := EncryptStringWithPassword(string(data), password)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(encrypted), 0o600)
}

// LoadEncryptedCredentialStoreWithPassword decrypts credentials produced by SaveEncryptedWithPassword.
func LoadEncryptedCredentialStoreWithPassword(path string, password string) (*CredentialStore, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read encrypted credentials: %w", err)
	}
	decrypted, err := DecryptStringWithPassword(string(payload), password)
	if err != nil {
		return nil, err
	}
	var store CredentialStore
	if err := json.Unmarshal([]byte(decrypted), &store); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if store.Credentials == nil {
		store.Credentials = map[string]Credential{}
	}
	return &store, nil
}

// GenerateEncryptionKey returns a random 32-byte hex-encoded key.
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, derivedKeyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return hex.EncodeToString(key), nil
}

func encryptAESGCM(plaintext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

func decryptAESGCM(ciphertext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
