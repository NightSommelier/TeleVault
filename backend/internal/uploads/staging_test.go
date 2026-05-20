package uploads

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalSpoolWriteCommitsFile(t *testing.T) {
	spool, err := NewLocalSpool(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}

	if err := spool.Write(context.Background(), "upload-1/part-1.age", func(w io.Writer) error {
		_, err := w.Write([]byte("ciphertext"))
		return err
	}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(spool.root, "upload-1", "part-1.age"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "ciphertext" {
		t.Fatalf("staged data = %q, want ciphertext", string(got))
	}
}

func TestLocalSpoolWriteRemovesTempOnError(t *testing.T) {
	root := t.TempDir()
	spool, err := NewLocalSpool(root)
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}

	writeErr := errors.New("write failed")
	if err := spool.Write(context.Background(), "upload-1/part-1.age", func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return writeErr
	}); !errors.Is(err, writeErr) {
		t.Fatalf("Write() error = %v, want writeErr", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, "upload-1"))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("spool dir entries = %d, want 0", len(entries))
	}
}

func TestLocalSpoolDeleteIgnoresMissingFile(t *testing.T) {
	spool, err := NewLocalSpool(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}

	if err := spool.Delete("missing/part.age"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestLocalSpoolDeleteRemovesEmptyUploadDirectory(t *testing.T) {
	root := t.TempDir()
	spool, err := NewLocalSpool(root)
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}
	if err := spool.Write(context.Background(), "upload-1/part-1.age", func(w io.Writer) error {
		_, err := w.Write([]byte("ciphertext"))
		return err
	}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := spool.Delete("upload-1/part-1.age"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "upload-1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("upload dir stat error = %v, want os.ErrNotExist", err)
	}
}

func TestCleanStorageKeyRejectsTraversal(t *testing.T) {
	if _, err := cleanStorageKey("../secret"); err == nil {
		t.Fatal("cleanStorageKey() accepted traversal")
	}
}
