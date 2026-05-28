package uploads

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NightSommelier/TeleVault/backend/internal/auth"
	"github.com/NightSommelier/TeleVault/backend/internal/telegramartifact"
)

const (
	defaultRetryBaseDelay = 30 * time.Second
	defaultRetryMaxDelay  = 30 * time.Minute
	slowdownRetryDelay    = 2 * time.Minute
)

var floodWaitPattern = regexp.MustCompile(`(?i)FLOOD_WAIT_?(\d+)`)

type WorkStore interface {
	ClaimQueuedPartWork(ctx context.Context, params ClaimQueuedPartParams) (QueuedPartWork, error)
	MarkStagedPartUploaded(ctx context.Context, params MarkStagedPartUploadedParams) (UploadPart, error)
	CompleteUpload(ctx context.Context, params CompleteUploadParams) (File, error)
	MarkLocalStagingDeleted(ctx context.Context, partID string) error
	RetryQueuedPart(ctx context.Context, params RetryPartParams) error
	FailQueuedPart(ctx context.Context, partID string, failure error) error
}

type WorkerSettings struct {
	WorkerID       string
	LeaseDuration  time.Duration
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	UploadTimeout  time.Duration
	Logger         *slog.Logger
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
	if settings.Logger == nil {
		settings.Logger = slog.Default()
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
	work, err := w.claimQueuedPartWork(ctx)
	if errors.Is(err, ErrUploadPartNotFound) {
		w.settings.Logger.Debug("worker queue empty")
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := w.drainClaimedWork(ctx, work); err != nil {
		return true, err
	}
	return true, nil
}

func (w *DrainWorker) DrainLoop(ctx context.Context, pollInterval time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	state := struct {
		sync.Mutex
		active int
		wake   chan struct{}
	}{
		wake: make(chan struct{}, 1),
	}
	notify := func() {
		select {
		case state.wake <- struct{}{}:
		default:
		}
	}

	launch := func(work QueuedPartWork) {
		state.Lock()
		state.active++
		state.Unlock()

		go func(work QueuedPartWork) {
			defer func() {
				state.Lock()
				state.active--
				state.Unlock()
				notify()
			}()
			if err := w.drainClaimedWork(ctx, work); err != nil {
				w.settings.Logger.Warn(
					"upload part drain failed",
					"part_id", work.Part.ID,
					"upload_id", work.Part.UploadID,
					"part_number", work.Part.PartNumber,
					"error", err,
				)
			}
		}(work)
	}

	var pending *QueuedPartWork
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		for {
			if pending == nil {
				work, err := w.claimQueuedPartWork(ctx)
				if errors.Is(err, ErrUploadPartNotFound) {
					break
				}
				if err != nil {
					return err
				}
				pending = &work
			}

			limit := pending.MaxParallelUploads
			if limit <= 0 {
				limit = 1
			}
			state.Lock()
			active := state.active
			state.Unlock()
			if active >= limit {
				break
			}

			launch(*pending)
			pending = nil
		}

		state.Lock()
		active := state.active
		state.Unlock()
		if active == 0 {
			timer := time.NewTimer(pollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-state.wake:
		}
	}
}

func (w *DrainWorker) claimQueuedPartWork(ctx context.Context) (QueuedPartWork, error) {
	now := w.settings.Now()
	return w.store.ClaimQueuedPartWork(ctx, ClaimQueuedPartParams{
		WorkerID:      w.settings.WorkerID,
		Now:           now,
		LeaseDuration: w.settings.LeaseDuration,
	})
}

func (w *DrainWorker) drainClaimedWork(ctx context.Context, work QueuedPartWork) error {
	part := work.Part
	w.settings.Logger.Debug(
		"upload part drain started",
		"part_id", part.ID,
		"upload_id", part.UploadID,
		"part_number", part.PartNumber,
		"attempts", part.Attempts,
		"storage_backend", nullableString(part.StorageBackend),
	)
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
	startedAt := w.settings.Now()
	artifactSize, err := uploadPartPlaintextSize(part)
	if err != nil {
		_ = w.store.FailQueuedPart(ctx, part.ID, err)
		return err
	}
	wrappedBody := telegramartifact.WrapReaderForSize(part.ID, artifactSize, body)

	result, err := w.telegram.UploadEncryptedPart(
		uploadCtx,
		session,
		nullableString(work.StoragePeer),
		telegramArtifactName(part.ID, artifactSize),
		telegramArtifactMimeType(part.ID, artifactSize),
		wrappedBody,
	)
	if err != nil {
		w.settings.Logger.Debug("telegram upload part failed", "part_id", part.ID, "upload_id", part.UploadID, "error", err)
		return w.retryPart(ctx, part, err)
	}

	if _, err := w.store.MarkStagedPartUploaded(ctx, MarkStagedPartUploadedParams{
		PartID:       part.ID,
		TelegramPeer: result.Peer,
		MessageID:    result.MessageID,
	}); err != nil {
		return err
	}

	if err := w.spool.Delete(part.StorageKey.String); err != nil {
		return err
	}
	if err := w.store.MarkLocalStagingDeleted(ctx, part.ID); err != nil {
		return err
	}
	if err := w.tryCompleteUpload(ctx, work); err != nil {
		return err
	}

	w.settings.Logger.Debug(
		"upload part drain completed",
		"part_id", part.ID,
		"upload_id", part.UploadID,
		"part_number", part.PartNumber,
		"telegram_peer", result.Peer,
		"telegram_message_id", result.MessageID,
		"duration_ms", w.settings.Now().Sub(startedAt).Milliseconds(),
	)
	return w.throttleAfterUpload(ctx, work, startedAt)
}

func (w *DrainWorker) tryCompleteUpload(ctx context.Context, work QueuedPartWork) error {
	file, err := w.store.CompleteUpload(ctx, CompleteUploadParams{
		OwnerID:  work.OwnerID,
		UploadID: work.Part.UploadID,
		Now:      w.settings.Now(),
	})
	if errors.Is(err, ErrUploadIncomplete) {
		w.settings.Logger.Debug(
			"upload auto-complete waiting for remaining parts",
			"upload_id", work.Part.UploadID,
			"part_id", work.Part.ID,
		)
		return nil
	}
	if errors.Is(err, ErrUploadNotFound) || errors.Is(err, ErrUploadExpired) {
		w.settings.Logger.Debug(
			"upload auto-complete skipped",
			"upload_id", work.Part.UploadID,
			"part_id", work.Part.ID,
			"error", err,
		)
		return nil
	}
	if errors.Is(err, ErrUploadSizeMismatch) || errors.Is(err, ErrUploadChecksumMismatch) {
		_ = w.store.FailQueuedPart(ctx, work.Part.ID, err)
		return err
	}
	if err != nil {
		return err
	}

	w.settings.Logger.Info(
		"upload auto-completed",
		"upload_id", work.Part.UploadID,
		"file_id", file.ID,
		"owner_id", work.OwnerID,
	)
	return nil
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
	if isTelegramSlowdown(err) && delay < slowdownRetryDelay {
		delay = slowdownRetryDelay
	}
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func isTelegramSlowdown(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "timeout") ||
		strings.Contains(message, "too many requests") ||
		strings.Contains(message, "rate limit") ||
		strings.Contains(message, "upload freeze") ||
		strings.Contains(message, "temporarily unavailable")
}

func (w *DrainWorker) throttleAfterUpload(ctx context.Context, work QueuedPartWork, startedAt time.Time) error {
	delay := uploadPolicyDelay(work, w.settings.Now().Sub(startedAt))
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil
	case <-timer.C:
		return nil
	}
}

func uploadPolicyDelay(work QueuedPartWork, elapsed time.Duration) time.Duration {
	delay := time.Duration(work.CooldownBetweenPartsMillisec) * time.Millisecond
	if work.TargetUploadBytesPerSecond <= 0 || !work.Part.CiphertextSize.Valid || work.Part.CiphertextSize.Int64 <= 0 {
		return delay
	}

	targetDuration := time.Duration(float64(work.Part.CiphertextSize.Int64) / float64(work.TargetUploadBytesPerSecond) * float64(time.Second))
	if targetDuration <= elapsed {
		return delay
	}
	return delay + targetDuration - elapsed
}
