package store

import (
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("SNAP_USER_COMMON", t.TempDir())
	s, err := New()
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return s
}

func TestReadRaw_missingFile(t *testing.T) {
	s := newTestStore(t)
	b, err := s.ReadRaw()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(b) != "[]" {
		t.Errorf("expected [], got %s", b)
	}
}

func TestWriteRead_roundtrip(t *testing.T) {
	s := newTestStore(t)
	data := []byte(`[{"id":"abc123","name":"test"}]`)
	if err := s.WriteRaw(data); err != nil {
		t.Fatalf("WriteRaw: %v", err)
	}
	got, err := s.ReadRaw()
	if err != nil {
		t.Fatalf("ReadRaw: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("expected %s, got %s", data, got)
	}
}

func TestWriteRaw_badPath(t *testing.T) {
	s := &Store{path: "/no/such/directory/file.json"}
	if err := s.WriteRaw([]byte("data")); err == nil {
		t.Error("expected error writing to invalid path")
	}
}
