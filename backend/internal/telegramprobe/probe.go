package telegramprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/auth"
)

const (
	DefaultMinBytes  = int64(1024 * 1024)
	DefaultStepBytes = int64(1024 * 1024)
)

var ErrInvalidPlan = errors.New("invalid telegram probe plan")

type UploadClient interface {
	UploadEncryptedPart(ctx context.Context, session string, storagePeer string, name string, body io.Reader) (auth.TelegramUploadResult, error)
	DeleteEncryptedPart(ctx context.Context, session string, storagePeer string, messageID int64) error
}

type Plan struct {
	MinBytes  int64
	MaxBytes  int64
	StepBytes int64
}

type Result struct {
	AttemptedSizes []int64
	DetectedBytes  int64
	FailedBytes    int64
	DryRun         bool
}

func (p Plan) Validate() error {
	if p.MinBytes <= 0 || p.MaxBytes <= 0 || p.StepBytes <= 0 || p.MinBytes > p.MaxBytes {
		return ErrInvalidPlan
	}
	return nil
}

func (p Plan) Sizes() ([]int64, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	var sizes []int64
	for size := p.MinBytes; size <= p.MaxBytes; size += p.StepBytes {
		sizes = append(sizes, size)
		if p.MaxBytes-size < p.StepBytes {
			break
		}
	}
	if sizes[len(sizes)-1] != p.MaxBytes {
		sizes = append(sizes, p.MaxBytes)
	}
	return sizes, nil
}

func DryRun(p Plan) (Result, error) {
	sizes, err := p.Sizes()
	if err != nil {
		return Result{}, err
	}
	return Result{AttemptedSizes: sizes, DryRun: true}, nil
}

func Run(ctx context.Context, client UploadClient, session string, storagePeer string, p Plan) (Result, error) {
	sizes, err := p.Sizes()
	if err != nil {
		return Result{}, err
	}
	if client == nil {
		return Result{}, errors.New("telegram upload client is required")
	}

	result := Result{AttemptedSizes: sizes}
	for _, size := range sizes {
		name := fmt.Sprintf("t2d-limit-probe-%d.bin", size)
		uploadResult, err := client.UploadEncryptedPart(ctx, session, storagePeer, name, io.LimitReader(zeroReader{}, size))
		if err != nil {
			result.FailedBytes = size
			return result, err
		}

		result.DetectedBytes = size
		if uploadResult.MessageID > 0 {
			deleteCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			deleteErr := client.DeleteEncryptedPart(deleteCtx, session, uploadResult.Peer, uploadResult.MessageID)
			cancel()
			if deleteErr != nil {
				return result, fmt.Errorf("probe upload succeeded at %d bytes but cleanup failed: %w", size, deleteErr)
			}
		}
	}

	return result, nil
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
