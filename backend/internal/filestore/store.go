package filestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Object struct {
	Key    string
	Path   string
	Bytes  int64
	SHA256 string
}

type Store interface {
	Backend() string
	Put(ctx context.Context, key string, body io.Reader) (*Object, error)
	Get(ctx context.Context, path string) ([]byte, error)
	Delete(ctx context.Context, path string) error
}

type LocalStore struct {
	dir string
}

func NewLocalStore(dir string) *LocalStore {
	if strings.TrimSpace(dir) == "" {
		dir = "/tmp/ai-aggregator-files"
	}
	return &LocalStore{dir: dir}
}

func (s *LocalStore) Backend() string {
	return "local"
}

func (s *LocalStore) Put(_ context.Context, key string, body io.Reader) (*Object, error) {
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("object key is required")
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return nil, fmt.Errorf("create local file store dir: %w", err)
	}
	path := filepath.Join(s.dir, filepath.Base(key))
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create local object: %w", err)
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(dst, io.TeeReader(body, hasher))
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if copyErr != nil {
			return nil, fmt.Errorf("write local object: %w", copyErr)
		}
		return nil, fmt.Errorf("close local object: %w", closeErr)
	}
	return &Object{
		Key:    key,
		Path:   path,
		Bytes:  written,
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}, nil
}

func (s *LocalStore) Get(_ context.Context, path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("object path is required")
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read local object: %w", err)
	}
	return bytes, nil
}

func (s *LocalStore) Delete(_ context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete local object: %w", err)
	}
	return nil
}
