package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TokenStore struct {
	encryptionKey []byte
	filePath      string
	mutex         sync.RWMutex
}

type DropboxTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
}

var (
	tokenStore *TokenStore
	once       sync.Once
)

// GetTokenStore returns a singleton instance of TokenStore
func GetTokenStore() (*TokenStore, error) {
	var initErr error
	once.Do(func() {
		// Create tokens directory if it doesn't exist
		tokenDir := "./tokens"
		if err := os.MkdirAll(tokenDir, 0700); err != nil {
			initErr = fmt.Errorf("failed to create token directory: %v", err)
			return
		}

		// In production, this key should be securely stored and retrieved
		// For now, we'll generate a new key if it doesn't exist
		keyPath := filepath.Join(tokenDir, "encryption.key")
		key, err := loadOrGenerateKey(keyPath)
		if err != nil {
			initErr = fmt.Errorf("failed to initialize encryption key: %v", err)
			return
		}

		tokenStore = &TokenStore{
			encryptionKey: key,
			filePath:      filepath.Join(tokenDir, "dropbox_tokens.enc"),
		}
	})

	if initErr != nil {
		return nil, initErr
	}
	return tokenStore, nil
}

func loadOrGenerateKey(keyPath string) ([]byte, error) {
	key := make([]byte, 32) // AES-256

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// Generate new key
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("failed to generate key: %v", err)
		}
		// Save key
		if err := os.WriteFile(keyPath, key, 0600); err != nil {
			return nil, fmt.Errorf("failed to save key: %v", err)
		}
	} else {
		// Load existing key
		var err error
		key, err = os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read key: %v", err)
		}
	}

	return key, nil
}

func (ts *TokenStore) SaveTokens(tokens *DropboxTokens) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	// Convert tokens to JSON
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %v", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(ts.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM cipher mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to create nonce: %v", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// Save to file
	if err := os.WriteFile(ts.filePath, ciphertext, 0600); err != nil {
		return fmt.Errorf("failed to save tokens: %v", err)
	}

	return nil
}

func (ts *TokenStore) LoadTokens() (*DropboxTokens, error) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	// Check if file exists
	if _, err := os.Stat(ts.filePath); os.IsNotExist(err) {
		return nil, nil
	}

	// Read encrypted data
	ciphertext, err := os.ReadFile(ts.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tokens: %v", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(ts.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM cipher mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	// Extract nonce and decrypt
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt tokens: %v", err)
	}

	// Parse JSON
	var tokens DropboxTokens
	if err := json.Unmarshal(plaintext, &tokens); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tokens: %v", err)
	}

	return &tokens, nil
}

func (ts *TokenStore) DeleteTokens() error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if err := os.Remove(ts.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete tokens: %v", err)
	}

	return nil
}
