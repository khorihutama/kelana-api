package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const nonceSize = 12

// Local stores AES-256-GCM encrypted blobs below root. The nonce is prefixed to
// every ciphertext; the object key is bound as additional authenticated data.
type Local struct {
	root string
	key  []byte
}

func NewLocal(root string, key []byte) (*Local, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("document encryption key must be 32 bytes")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create document root: %w", err)
	}
	return &Local{root: root, key: append([]byte(nil), key...)}, nil
}

// Put encrypts plaintext and returns the SHA-256 checksum of the plaintext.
func (l *Local) Put(ctx context.Context, key string, plaintext []byte) (string, error) {
	path, err := l.path(key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create document dir: %w", err)
	}

	sum := sha256.Sum256(plaintext)
	aead, err := l.aead()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := aead.Seal(nil, nonce, plaintext, []byte(key))

	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := tmp.Write(nonce); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write nonce: %w", err)
	}
	if _, err := tmp.Write(sealed); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write ciphertext: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("sync document: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close document: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("commit document: %w", err)
	}
	return hex.EncodeToString(sum[:]), nil
}

func (l *Local) Get(ctx context.Context, key string) ([]byte, error) {
	path, err := l.path(key)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) <= nonceSize {
		return nil, errors.New("document ciphertext is truncated")
	}
	aead, err := l.aead()
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, raw[:nonceSize], raw[nonceSize:], []byte(key))
	if err != nil {
		return nil, fmt.Errorf("decrypt document: %w", err)
	}
	return plaintext, nil
}

func (l *Local) Delete(ctx context.Context, key string) error {
	path, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (l *Local) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(l.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (l *Local) path(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", errors.New("empty object key")
	}
	clean := filepath.Clean(filepath.FromSlash(key))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid object key %q", key)
	}
	return filepath.Join(l.root, clean), nil
}
