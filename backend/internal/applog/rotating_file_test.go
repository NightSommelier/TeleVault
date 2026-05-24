package applog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotatingFileWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api.log")
	writer, err := newRotatingFileWriter(path, 16, 2)
	if err != nil {
		t.Fatalf("newRotatingFileWriter: %v", err)
	}
	if _, err := writer.Write([]byte("1234567890")); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if _, err := writer.Write([]byte("abcdefghij")); err != nil {
		t.Fatalf("write second: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated file: %v", err)
	}
}
