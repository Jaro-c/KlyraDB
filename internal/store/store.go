package store

import (
	"os"
	"path/filepath"
)

type Store struct {
	path string
}

func New() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".local", "share", "klyradb")
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
