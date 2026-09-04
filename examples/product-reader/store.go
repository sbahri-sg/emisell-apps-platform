package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type session struct {
	CSRF, PendingRequest, StateHash, Verifier       string
	Token, InstallationID, MerchantID, MerchantName string
	ExpiresAt, StateExpiresAt, TokenExpiresAt       time.Time
	Notice                                          string
}

// Single-process example store: encrypted at rest, opaque hashed cookie keys, atomic writes.
// A production provider should use its own user-bound durable secret store instead.
type store struct {
	mu    sync.Mutex
	path  string
	aead  cipher.AEAD
	lock  *os.File
	items map[string]session
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func openStore(path string, key []byte) (*store, error) {
	if len(key) != 32 || !filepath.IsAbs(path) {
		return nil, errors.New("absolute store path and 32-byte encryption key required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, errors.New("example store is already in use")
	}
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	s := &store{path: path, aead: aead, lock: lock, items: map[string]session{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil || len(data) > 8*1024*1024 || len(data) < aead.NonceSize() {
		s.Close()
		return nil, errors.New("invalid encrypted store")
	}
	plain, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte("product-reader-v1"))
	if err != nil || json.Unmarshal(plain, &s.items) != nil || s.items == nil {
		s.Close()
		return nil, errors.New("cannot decrypt example store")
	}
	return s, nil
}

func (s *store) Close() { _ = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN); _ = s.lock.Close() }

func (s *store) get(id string) (session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[digest(id)]
	return item, ok && item.ExpiresAt.After(time.Now())
}

func (s *store) change(id string, change func(*session) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Work on a copy so failed writes never leave an unpersisted token/state in memory.
	items := make(map[string]session, len(s.items)+1)
	for key, item := range s.items {
		if item.ExpiresAt.After(time.Now()) {
			items[key] = item
		}
	}
	key := digest(id)
	item := items[key]
	if err := change(&item); err != nil {
		return err
	}
	if len(items) >= 1000 {
		if _, exists := items[key]; !exists {
			return errors.New("session capacity reached")
		}
	}
	items[key] = item
	plain, err := json.Marshal(items)
	if err != nil || len(plain) > 4*1024*1024 {
		return errors.New("store capacity reached")
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := s.aead.Seal(nonce, nonce, plain, []byte("product-reader-v1"))
	file, err := os.CreateTemp(filepath.Dir(s.path), ".reader-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(sealed); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), s.path)
	}
	if err != nil {
		return err
	}
	s.items = items
	return nil
}
