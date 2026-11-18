package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T, key byte) *Local {
	t.Helper()
	store, err := NewLocal(t.TempDir(), bytes.Repeat([]byte{key}, 32))
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return store
}

func TestLocalRoundTripEncryptsAtRest(t *testing.T) {
	store := newStore(t, 7)
	plaintext := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("payload"), 512)...)
	key := "reservations/res-1/doc-1.enc"

	checksum, err := store.Put(context.Background(), key, plaintext)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	sum := sha256.Sum256(plaintext)
	if checksum != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %s, want %s", checksum, hex.EncodeToString(sum[:]))
	}

	raw, err := os.ReadFile(filepath.Join(store.root, filepath.FromSlash(key)))
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if bytes.HasPrefix(raw, []byte("%PDF")) {
		t.Fatal("stored file is not encrypted")
	}
	if len(raw) != len(plaintext)+nonceSize+16 {
		t.Fatalf("stored size = %d, want %d", len(raw), len(plaintext)+nonceSize+16)
	}

	got, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatal("decrypted payload differs from original")
	}
}

func TestLocalRejectsTamperedCiphertext(t *testing.T) {
	store := newStore(t, 9)
	key := "reservations/res-1/doc-1.enc"
	if _, err := store.Put(context.Background(), key, []byte("secret document")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	path := filepath.Join(store.root, filepath.FromSlash(key))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write tampered: %v", err)
	}
	if _, err := store.Get(context.Background(), key); err == nil {
		t.Fatal("Get accepted tampered ciphertext")
	}
}

func TestLocalWrongKeyAndTraversal(t *testing.T) {
	dir := t.TempDir()
	first, err := NewLocal(dir, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	key := "reservations/res-1/doc-1.enc"
	if _, err := first.Put(context.Background(), key, []byte("secret")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	second, err := NewLocal(dir, bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	if _, err := second.Get(context.Background(), key); err == nil {
		t.Fatal("Get with a different key succeeded")
	}
	if _, err := first.Put(context.Background(), "../escape.enc", []byte("x")); err == nil {
		t.Fatal("Put accepted a traversal key")
	}
}
