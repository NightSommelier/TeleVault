package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type rotatingFileWriter struct {
	path       string
	maxBytes   int64
	maxBackups int
	file       *os.File
	size       int64
	mu         sync.Mutex
}

func newRotatingFileWriter(path string, maxBytes int64, maxBackups int) (*rotatingFileWriter, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("max bytes must be greater than 0")
	}
	if maxBackups <= 0 {
		return nil, fmt.Errorf("max backups must be greater than 0")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &rotatingFileWriter{
		path:       path,
		maxBytes:   maxBytes,
		maxBackups: maxBackups,
		file:       f,
		size:       stat.Size(),
	}, nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFileWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	for i := w.maxBackups; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", w.path, i)
		dst := fmt.Sprintf("%s.%d", w.path, i+1)
		if i == w.maxBackups {
			_ = os.Remove(src)
			continue
		}
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return err
			}
		}
	}
	if _, err := os.Stat(w.path); err == nil {
		if err := os.Rename(w.path, fmt.Sprintf("%s.1", w.path)); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.size = 0
	return nil
}
