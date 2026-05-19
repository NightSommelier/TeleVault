package uploads

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleDriveVault/backend/internal/auth"
)

const (
	defaultRetryBaseDelay = 30 * time.Second
	defaultRetryMaxDelay  = 30 * time.Minute
)

var floodWaitPattern = regexp.MustCompile(`(?i)FLOOD_WAIT_?(\d+)`)

type WorkStore interface {
	ClaimQueuedPartWork(ctx context.Context, params ClaimQueuedPartParams) (QueuedPartWork, error)
	MarkStagedPartUploaded(ctx context.Context, params MarkStagedPartUploadedParams) (UploadPart, error)
	RetryQueuedPart(ctx context.Context, params RetryPartParams) error
	FailQueuedPart(ctx context.Context, partID string, failure error) error
}

type WorkerSettings struct {
	WorkerID       string
	LeaseDuration  time.Duration
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	UploadTimeout  time.Duration
	Now            func() time.Time
}

type DrainWorker struct {
	store         WorkStore
	spool         *LocalSpool
	sessionCrypto auth.TelegramSessionCrypto
	telegram      auth.TelegramStorageClient
	settings      WorkerSettings
}

func NewDrainWorker(store WorkStore, spool *LocalSpool, sessionCrypto auth.TelegramSessionCrypto, telegram auth.TelegramStorageClient, settings WorkerSettings) (*DrainWorker, error) {
	if store == nil {
		return nil, errors.New("store is required")
	}
	if spool == nil {
		return nil, errors.New("spool is required")
	}
	if telegram == nil {
		return nil, errors.New("telegram client is required")
	}
	if settings.WorkerID == "" {
		return nil, errors.New("worker id is required")
	}
	if settings.LeaseDuration <= 0 {
		settings.LeaseDuration = 5 * time.Minute
	}
	if settings.RetryBaseDelay <= 0 {
		settings.RetryBaseDelay = defaultRetryBaseDelay
	}
	if settings.RetryMaxDelay <= 0 {
		settings.RetryMaxDelay = defaultRetryMaxDelay
	}
	if settings.UploadTimeout <= 0 {
		settings.UploadTimeout = 30 * time.Minute
	}
	if settings.Now == nil {
		settings.Now = time.Now
	}

	return &DrainWorker{
		store:         store,
		spool:         spool,
		sessionCrypto: sessionCrypto,
		telegram:      telegram,
		settings:      settings,
	}, nil
}

func (w *DrainWorker) DrainOne(ctx context.Context) (bool, error) {
	now := w.settings.Now()
	work, err := w.store.ClaimQueuedPartWork(ctx, ClaimQueuedPartParams{
		WorkerID:      w.settings.WorkerID,
		Now:           now,
		LeaseDuration: w.settings.LeaseDuration,
	})
	if errors.Is(err, ErrUploadPartNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := w.drainClaimedPart(ctx, work); err != nil {
		return true, err
	}
	return true, nil
}

func (w *DrainWorker) drainClaimedPart(ctx context.Context, work QueuedPartWork) error {
	part := work.Part
	if !part.StorageBackend.Valid || part.StorageBackend.String != LocalStagingBackend {
		err := fmt.Errorf("unsupported storage backend %q", nullableString(part.StorageBackend))
		_ = w.store.FailQueuedPart(ctx, part.ID, err)
		return err
	}
	if !part.StorageKey.Valid {
		err := errors.New("staged part storage key is missing")
		_ = w.store.FailQueuedPart(ctx, part.ID, err)
		return err
	}

	session, err := w.sessionCrypto.DecryptForTelegramID(work.OwnerTelegramID, work.EncryptedSession)
	if err != nil {
		_ = w.store.FailQueuedPart(ctx, part.ID, err)
		return err
	}

	body, err := w.spool.Open(part.StorageKey.String)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = w.store.FailQueuedPart(ctx, part.ID, err)
			return err
		}
		return w.retryPart(ctx, part, err)
	}
	defer body.Close()

	uploadCtx, cancel := context.WithTimeout(ctx, w.settings.UploadTimeout)
	defer cancel()

	result, err := w.telegram.UploadEncryptedPart(
		uploadCtx,
		session,
		nullableString(work.StoragePeer),
		telegramArtifactName(part.ID),
		body,
	)
	if err != nil {
		return w.retryPart(ctx, part, err)
	}

	if _, err := w.store.MarkStagedPartUploaded(ctx, MarkStagedPartUploadedParams{
		PartID:       part.ID,
		TelegramPeer: result.Peer,
		MessageID:    result.MessageID,
	}); err != nil {
		return err
	}

	return w.spool.Delete(part.StorageKey.String)
}

func (w *DrainWorker) retryPart(ctx context.Context, part UploadPart, cause error) error {
	delay := retryDelay(cause, part.Attempts, w.settings.RetryBaseDelay, w.settings.RetryMaxDelay)
	if err := w.store.RetryQueuedPart(ctx, RetryPartParams{
		PartID:      part.ID,
		LastError:   cause.Error(),
		AvailableAt: w.settings.Now().Add(delay),
	}); err != nil {
		return err
	}
	return cause
}

func retryDelay(err error, attempts int, base time.Duration, maxDelay time.Duration) time.Duration {
	if err != nil {
		if match := floodWaitPattern.FindStringSubmatch(err.Error()); len(match) == 2 {
			seconds, parseErr := strconv.Atoi(match[1])
			if parseErr == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}

	if attempts < 1 {
		attempts = 1
	}
	power := attempts - 1
	if power > 10 {
		power = 10
	}
	delay := time.Duration(math.Pow(2, float64(power))) * base
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}
