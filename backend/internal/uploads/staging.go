package uploads

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const LocalStagingBackend = "local"

type LocalSpool struct {
	root string
}

func NewLocalSpool(root string) (*LocalSpool, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("local spool root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	return &LocalSpool{root: root}, nil
}

func (s *LocalSpool) Write(ctx context.Context, key string, write func(io.Writer) error) error {
	if s == nil {
		return errors.New("local spool is nil")
	}
	key, err := cleanStorageKey(key)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	path := filepath.Join(s.root, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".part-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	if err := write(tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *LocalSpool) Delete(key string) error {
	if s == nil {
		return errors.New("local spool is nil")
	}
	key, err := cleanStorageKey(key)
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(s.root, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *LocalSpool) Open(key string) (io.ReadCloser, error) {
	if s == nil {
		return nil, errors.New("local spool is nil")
	}
	key, err := cleanStorageKey(key)
	if err != nil {
		return nil, err
	}
	return os.Open(filepath.Join(s.root, key))
}

func cleanStorageKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("storage key is required")
	}
	cleaned := filepath.Clean(key)
	if filepath.IsAbs(cleaned) || cleaned == "." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return "", errors.New("storage key must be relative")
	}
	return cleaned, nil
}

func stagedPartKey(uploadID string, partNumber int) string {
	return filepath.Join(uploadID, "part-"+strconv.Itoa(partNumber)+".age")
}
