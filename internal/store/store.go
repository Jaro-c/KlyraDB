package store

import (
	"os"
	"path/filepath"

	"klyradb/internal/engine"
)

type Store struct {
	path string
}

func New() (*Store, error) {
	dir := engine.BaseDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(dir, "instances.json")}, nil
}

func (s *Store) ReadRaw() ([]byte, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte("[]"), nil
		}
		return nil, err
	}
	return b, nil
}

func (s *Store) WriteRaw(b []byte) error {
	return os.WriteFile(s.path, b, 0o644)
}
