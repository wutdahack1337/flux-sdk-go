package types

import (
	fmt "fmt"
	io "io"
	"os"
)

type WAL interface {
	Write(height uint64, b []byte) error
	Read(height uint64) ([]byte, error)
	Prune(currentHeight uint64) error
	Close()
}

type SimpleFileWAL struct {
	f *os.File
}

var _ WAL = &SimpleFileWAL{}

func NewSimpleFileWAL(name string) (*SimpleFileWAL, error) {
	file, err := os.OpenFile(name, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	return &SimpleFileWAL{f: file}, nil
}

func (w *SimpleFileWAL) Write(height uint64, b []byte) error {
	_, err := w.f.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek in WAL: %w", err)
	}

	err = w.f.Truncate(0)
	if err != nil {
		return fmt.Errorf("failed to truncate WAL: %w", err)
	}

	n, err := w.f.Write(b)
	if err != nil {
		return fmt.Errorf("failed to write WAL: %w", err)
	}

	if n != len(b) {
		return fmt.Errorf("write len mismatch: written %d != requested: %d", n, len(b))
	}

	return w.f.Sync()
}

func (w *SimpleFileWAL) Read(height uint64) ([]byte, error) {
	_, err := w.f.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("seek err: %w", err)
	}

	bz, err := io.ReadAll(w.f)
	if err != nil {
		return nil, err
	}

	return bz, nil
}

func (w *SimpleFileWAL) Close() {
	w.f.Close()
}

// no prune needed for simple-file-based WAL, as we rewrite everytime new height arrives
func (w *SimpleFileWAL) Prune(currentHeight uint64) error {
	return nil
}

type MockWAL struct {
	mem []byte
}

var _ WAL = &MockWAL{}

func NewMockWAL() *MockWAL {
	return &MockWAL{}
}

func (w *MockWAL) Write(height uint64, b []byte) error {
	w.mem = b
	return nil
}

func (w *MockWAL) Read(height uint64) ([]byte, error) {
	return w.mem, nil
}

func (w *MockWAL) Close() {
}

func (w *MockWAL) Prune(currentHeight uint64) error {
	return nil
}
