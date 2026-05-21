package uploads

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/secrets"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/telegramartifact"
)

func TestDrainWorkerUploadsStagedPartAndDeletesLocalCopy(t *testing.T) {
	ctx := context.Background()
	const artifactSize = int64(4 * 1024 * 1024)
	spool, err := NewLocalSpool(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}
	if err := spool.Write(ctx, "upload-1/part-1.age", func(w io.Writer) error {
		_, err := w.Write([]byte("ciphertext"))
		return err
	}); err != nil {
		t.Fatalf("spool.Write() error = %v", err)
	}

	crypto := testSessionCrypto(t)
	encryptedSession, err := crypto.Encrypt("telegram:12345", "telegram-session")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	store := &fakeWorkStore{
		work: QueuedPartWork{
			Part: UploadPart{
				ID:             "part-1",
				UploadID:       "upload-1",
				PartNumber:     1,
				PlaintextSize:  sql.NullInt64{Int64: artifactSize, Valid: true},
				StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
				StorageKey:     sql.NullString{String: "upload-1/part-1.age", Valid: true},
				Attempts:       1,
			},
			OwnerID:          "owner-1",
			OwnerTelegramID:  12345,
			EncryptedSession: encryptedSession,
			StoragePeer:      sql.NullString{String: "self", Valid: true},
		},
	}
	telegram := &fakeWorkerTelegram{messageID: 77}

	worker, err := NewDrainWorker(store, spool, crypto, telegram, WorkerSettings{
		WorkerID:      "worker-a",
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return time.Unix(1000, 0) },
	})
	if err != nil {
		t.Fatalf("NewDrainWorker() error = %v", err)
	}

	worked, err := worker.DrainOne(ctx)
	if err != nil {
		t.Fatalf("DrainOne() error = %v", err)
	}
	if !worked {
		t.Fatalf("DrainOne() worked = false, want true")
	}
	wantName := telegramArtifactName("part-1", artifactSize)
	wantMIME := telegramArtifactMimeType("part-1", artifactSize)
	wantBody, err := io.ReadAll(telegramartifact.WrapReaderForSize("part-1", artifactSize, bytes.NewReader([]byte("ciphertext"))))
	if err != nil {
		t.Fatalf("ReadAll(WrapReader()) error = %v", err)
	}
	if telegram.session != "telegram-session" || telegram.peer != "self" || telegram.name != wantName || telegram.mimeType != wantMIME || !bytes.Equal(telegram.body, wantBody) {
		t.Fatalf("telegram upload = session %q peer %q name %q mime %q body %q", telegram.session, telegram.peer, telegram.name, telegram.mimeType, string(telegram.body))
	}
	if store.marked.PartID != "part-1" || store.marked.TelegramPeer != "self" || store.marked.MessageID != 77 {
		t.Fatalf("MarkStagedPartUploaded() params = %+v", store.marked)
	}
	if store.localDeletedID != "part-1" {
		t.Fatalf("MarkLocalStagingDeleted() part ID = %q, want part-1", store.localDeletedID)
	}
	if store.completed.OwnerID != "owner-1" || store.completed.UploadID != "upload-1" || !store.completed.Now.Equal(time.Unix(1000, 0)) {
		t.Fatalf("CompleteUpload() params = %+v", store.completed)
	}
	if _, err := spool.Open("upload-1/part-1.age"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("spool.Open() after drain error = %v, want os.ErrNotExist", err)
	}
}

func TestDrainWorkerIgnoresIncompleteAutoComplete(t *testing.T) {
	ctx := context.Background()
	const artifactSize = int64(4 * 1024 * 1024)
	spool, err := NewLocalSpool(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}
	if err := spool.Write(ctx, "upload-1/part-1.age", func(w io.Writer) error {
		_, err := w.Write([]byte("ciphertext"))
		return err
	}); err != nil {
		t.Fatalf("spool.Write() error = %v", err)
	}

	crypto := testSessionCrypto(t)
	encryptedSession, err := crypto.Encrypt("telegram:12345", "telegram-session")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	store := &fakeWorkStore{
		work: QueuedPartWork{
			Part: UploadPart{
				ID:             "part-1",
				UploadID:       "upload-1",
				PartNumber:     1,
				PlaintextSize:  sql.NullInt64{Int64: artifactSize, Valid: true},
				StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
				StorageKey:     sql.NullString{String: "upload-1/part-1.age", Valid: true},
				Attempts:       1,
			},
			OwnerID:          "owner-1",
			OwnerTelegramID:  12345,
			EncryptedSession: encryptedSession,
		},
		completeErr: ErrUploadIncomplete,
	}
	telegram := &fakeWorkerTelegram{messageID: 77}

	worker, err := NewDrainWorker(store, spool, crypto, telegram, WorkerSettings{
		WorkerID:      "worker-a",
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return time.Unix(1000, 0) },
	})
	if err != nil {
		t.Fatalf("NewDrainWorker() error = %v", err)
	}

	worked, err := worker.DrainOne(ctx)
	if err != nil {
		t.Fatalf("DrainOne() error = %v", err)
	}
	if !worked {
		t.Fatalf("DrainOne() worked = false, want true")
	}
	if store.completed.UploadID != "upload-1" {
		t.Fatalf("CompleteUpload() params = %+v", store.completed)
	}
}

func TestDrainWorkerRetriesFloodWait(t *testing.T) {
	ctx := context.Background()
	const artifactSize = int64(4 * 1024 * 1024)
	spool, err := NewLocalSpool(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}
	if err := spool.Write(ctx, "upload-1/part-1.age", func(w io.Writer) error {
		_, err := w.Write([]byte("ciphertext"))
		return err
	}); err != nil {
		t.Fatalf("spool.Write() error = %v", err)
	}

	now := time.Unix(2000, 0)
	crypto := testSessionCrypto(t)
	encryptedSession, err := crypto.Encrypt("telegram:12345", "telegram-session")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	store := &fakeWorkStore{
		work: QueuedPartWork{
			Part: UploadPart{
				ID:             "part-1",
				UploadID:       "upload-1",
				PartNumber:     1,
				PlaintextSize:  sql.NullInt64{Int64: artifactSize, Valid: true},
				StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
				StorageKey:     sql.NullString{String: "upload-1/part-1.age", Valid: true},
				Attempts:       3,
			},
			OwnerTelegramID:  12345,
			EncryptedSession: encryptedSession,
		},
	}
	telegram := &fakeWorkerTelegram{err: errors.New("FLOOD_WAIT_42")}
	worker, err := NewDrainWorker(store, spool, crypto, telegram, WorkerSettings{
		WorkerID:       "worker-a",
		LeaseDuration:  time.Minute,
		RetryBaseDelay: time.Second,
		RetryMaxDelay:  time.Hour,
		Now:            func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewDrainWorker() error = %v", err)
	}

	worked, err := worker.DrainOne(ctx)
	if err == nil {
		t.Fatalf("DrainOne() error = nil, want upload error")
	}
	if !worked {
		t.Fatalf("DrainOne() worked = false, want true")
	}
	if store.retry.PartID != "part-1" || store.retry.LastError != "FLOOD_WAIT_42" || !store.retry.AvailableAt.Equal(now.Add(42*time.Second)) {
		t.Fatalf("RetryQueuedPart() params = %+v", store.retry)
	}
}

func TestDrainLoopRespectsConfiguredConcurrency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	spool, err := NewLocalSpool(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}
	for _, key := range []string{"upload-a/part-1.age", "upload-b/part-1.age"} {
		if err := spool.Write(ctx, key, func(w io.Writer) error {
			_, err := w.Write([]byte("ciphertext"))
			return err
		}); err != nil {
			t.Fatalf("spool.Write(%q) error = %v", key, err)
		}
	}

	crypto := testSessionCrypto(t)
	encryptedSession, err := crypto.Encrypt("telegram:12345", "telegram-session")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	store := &sequenceWorkStore{
		works: []QueuedPartWork{
			{
				Part: UploadPart{
					ID:             "part-a",
					UploadID:       "upload-a",
					PartNumber:     1,
					PlaintextSize:  sql.NullInt64{Int64: 4 * 1024 * 1024, Valid: true},
					StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
					StorageKey:     sql.NullString{String: "upload-a/part-1.age", Valid: true},
				},
				OwnerTelegramID:    12345,
				EncryptedSession:   encryptedSession,
				StoragePeer:        sql.NullString{String: "self", Valid: true},
				MaxParallelUploads: 2,
			},
			{
				Part: UploadPart{
					ID:             "part-b",
					UploadID:       "upload-b",
					PartNumber:     1,
					PlaintextSize:  sql.NullInt64{Int64: 4 * 1024 * 1024, Valid: true},
					StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
					StorageKey:     sql.NullString{String: "upload-b/part-1.age", Valid: true},
				},
				OwnerTelegramID:    12345,
				EncryptedSession:   encryptedSession,
				StoragePeer:        sql.NullString{String: "self", Valid: true},
				MaxParallelUploads: 2,
			},
		},
	}
	telegram := newGateWorkerTelegram(2)

	worker, err := NewDrainWorker(store, spool, crypto, telegram, WorkerSettings{
		WorkerID:      "worker-a",
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return time.Unix(1000, 0) },
	})
	if err != nil {
		t.Fatalf("NewDrainWorker() error = %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- worker.DrainLoop(ctx, 10*time.Millisecond)
	}()

	select {
	case <-telegram.started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for concurrent uploads to start")
	}
	if got := telegram.maxActive(); got != 2 {
		t.Fatalf("max active uploads = %d, want 2", got)
	}

	telegram.release()
	cancel()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("DrainLoop() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for DrainLoop() to stop")
	}
}

func TestDrainLoopCanUploadPartsFromSameUploadConcurrently(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	spool, err := NewLocalSpool(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}
	for _, key := range []string{"upload-a/part-1.age", "upload-a/part-2.age"} {
		if err := spool.Write(ctx, key, func(w io.Writer) error {
			_, err := w.Write([]byte("ciphertext"))
			return err
		}); err != nil {
			t.Fatalf("spool.Write(%q) error = %v", key, err)
		}
	}

	crypto := testSessionCrypto(t)
	encryptedSession, err := crypto.Encrypt("telegram:12345", "telegram-session")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	store := &sequenceWorkStore{
		works: []QueuedPartWork{
			testQueuedWork("part-a", "upload-a", "upload-a/part-1.age", encryptedSession, 2),
			testQueuedWork("part-b", "upload-a", "upload-a/part-2.age", encryptedSession, 2),
		},
	}
	telegram := newGateWorkerTelegram(2)

	worker, err := NewDrainWorker(store, spool, crypto, telegram, WorkerSettings{
		WorkerID:      "worker-a",
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return time.Unix(1000, 0) },
	})
	if err != nil {
		t.Fatalf("NewDrainWorker() error = %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- worker.DrainLoop(ctx, 10*time.Millisecond)
	}()

	select {
	case <-telegram.started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for same-upload parts to start")
	}
	if got := telegram.maxActive(); got != 2 {
		t.Fatalf("max active uploads = %d, want 2", got)
	}

	telegram.release()
	cancel()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("DrainLoop() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for DrainLoop() to stop")
	}
}

func TestDrainLoopDoesNotExceedLowerCapForPendingWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	spool, err := NewLocalSpool(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalSpool() error = %v", err)
	}
	for _, key := range []string{"upload-a/part-1.age", "upload-b/part-1.age", "upload-c/part-1.age"} {
		if err := spool.Write(ctx, key, func(w io.Writer) error {
			_, err := w.Write([]byte("ciphertext"))
			return err
		}); err != nil {
			t.Fatalf("spool.Write(%q) error = %v", key, err)
		}
	}

	crypto := testSessionCrypto(t)
	encryptedSession, err := crypto.Encrypt("telegram:12345", "telegram-session")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	store := &sequenceWorkStore{
		works: []QueuedPartWork{
			testQueuedWork("part-a", "upload-a", "upload-a/part-1.age", encryptedSession, 2),
			testQueuedWork("part-b", "upload-b", "upload-b/part-1.age", encryptedSession, 2),
			testQueuedWork("part-c", "upload-c", "upload-c/part-1.age", encryptedSession, 1),
		},
	}
	telegram := newGateWorkerTelegram(3)

	worker, err := NewDrainWorker(store, spool, crypto, telegram, WorkerSettings{
		WorkerID:      "worker-a",
		LeaseDuration: time.Minute,
		Now:           func() time.Time { return time.Unix(1000, 0) },
	})
	if err != nil {
		t.Fatalf("NewDrainWorker() error = %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- worker.DrainLoop(ctx, 10*time.Millisecond)
	}()

	select {
	case <-telegram.started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for two active uploads")
	}
	time.Sleep(50 * time.Millisecond)
	if got := telegram.uploadCount(); got != 2 {
		t.Fatalf("started uploads before release = %d, want 2", got)
	}

	telegram.release()
	cancel()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("DrainLoop() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for DrainLoop() to stop")
	}
	if got := telegram.uploadCount(); got != 3 {
		t.Fatalf("started uploads after release = %d, want 3", got)
	}
	if got := telegram.maxActive(); got != 2 {
		t.Fatalf("max active uploads = %d, want 2", got)
	}
}

func TestUploadPolicyDelayUsesCooldownAndTargetRate(t *testing.T) {
	delay := uploadPolicyDelay(QueuedPartWork{
		Part: UploadPart{
			CiphertextSize: sql.NullInt64{Int64: 100, Valid: true},
		},
		TargetUploadBytesPerSecond:   50,
		CooldownBetweenPartsMillisec: 250,
	}, 500*time.Millisecond)
	if delay != 1750*time.Millisecond {
		t.Fatalf("uploadPolicyDelay() = %v, want 1.75s", delay)
	}

	delay = uploadPolicyDelay(QueuedPartWork{
		CooldownBetweenPartsMillisec: 250,
	}, 10*time.Second)
	if delay != 250*time.Millisecond {
		t.Fatalf("uploadPolicyDelay() without target rate = %v, want cooldown only", delay)
	}
}

func TestRetryDelayTreatsTelegramSlowdownsConservatively(t *testing.T) {
	delay := retryDelay(context.DeadlineExceeded, 1, time.Second, time.Hour)
	if delay != slowdownRetryDelay {
		t.Fatalf("retryDelay(context deadline) = %v, want %v", delay, slowdownRetryDelay)
	}

	delay = retryDelay(errors.New("Timeout while fetching dc"), 1, time.Second, time.Hour)
	if delay != slowdownRetryDelay {
		t.Fatalf("retryDelay(timeout) = %v, want %v", delay, slowdownRetryDelay)
	}

	delay = retryDelay(errors.New("FLOOD_WAIT_42"), 1, time.Second, time.Hour)
	if delay != 42*time.Second {
		t.Fatalf("retryDelay(FLOOD_WAIT_42) = %v, want 42s", delay)
	}
}

func testSessionCrypto(t *testing.T) auth.TelegramSessionCrypto {
	t.Helper()
	box, err := secrets.NewBox(bytes.Repeat([]byte{1}, secrets.KeyBytes))
	if err != nil {
		t.Fatalf("NewBox() error = %v", err)
	}
	return auth.NewTelegramSessionCrypto(box)
}

type fakeWorkStore struct {
	work           QueuedPartWork
	claimed        bool
	marked         MarkStagedPartUploadedParams
	completed      CompleteUploadParams
	completeErr    error
	localDeletedID string
	retry          RetryPartParams
	failedID       string
	failedErr      error
}

func (s *fakeWorkStore) ClaimQueuedPartWork(context.Context, ClaimQueuedPartParams) (QueuedPartWork, error) {
	if s.claimed {
		return QueuedPartWork{}, ErrUploadPartNotFound
	}
	s.claimed = true
	return s.work, nil
}

func (s *fakeWorkStore) MarkStagedPartUploaded(_ context.Context, params MarkStagedPartUploadedParams) (UploadPart, error) {
	s.marked = params
	part := s.work.Part
	part.Status = StatusComplete
	part.TelegramPeer = sql.NullString{String: params.TelegramPeer, Valid: params.TelegramPeer != ""}
	part.MessageID = sql.NullInt64{Int64: params.MessageID, Valid: params.MessageID != 0}
	return part, nil
}

func (s *fakeWorkStore) CompleteUpload(_ context.Context, params CompleteUploadParams) (File, error) {
	s.completed = params
	if s.completeErr != nil {
		return File{}, s.completeErr
	}
	return File{ID: "file-1"}, nil
}

func (s *fakeWorkStore) MarkLocalStagingDeleted(_ context.Context, partID string) error {
	s.localDeletedID = partID
	return nil
}

func (s *fakeWorkStore) RetryQueuedPart(_ context.Context, params RetryPartParams) error {
	s.retry = params
	return nil
}

func (s *fakeWorkStore) FailQueuedPart(_ context.Context, partID string, failure error) error {
	s.failedID = partID
	s.failedErr = failure
	return nil
}

type fakeWorkerTelegram struct {
	messageID int64
	err       error
	session   string
	peer      string
	name      string
	mimeType  string
	body      []byte
}

func (t *fakeWorkerTelegram) UploadEncryptedPart(_ context.Context, session string, storagePeer string, name string, mimeType string, body io.Reader) (auth.TelegramUploadResult, error) {
	t.session = session
	t.peer = storagePeer
	t.name = name
	t.mimeType = mimeType
	var err error
	t.body, err = io.ReadAll(body)
	if err != nil {
		return auth.TelegramUploadResult{}, err
	}
	if t.err != nil {
		return auth.TelegramUploadResult{}, t.err
	}
	return auth.TelegramUploadResult{Peer: "self", MessageID: t.messageID}, nil
}

func (t *fakeWorkerTelegram) DownloadEncryptedPart(context.Context, string, string, int64, io.Writer) error {
	return nil
}

func (t *fakeWorkerTelegram) DeleteEncryptedPart(context.Context, string, string, int64) error {
	return nil
}

type sequenceWorkStore struct {
	mu        sync.Mutex
	works     []QueuedPartWork
	next      int
	completed []CompleteUploadParams
}

func (s *sequenceWorkStore) ClaimQueuedPartWork(context.Context, ClaimQueuedPartParams) (QueuedPartWork, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.next >= len(s.works) {
		return QueuedPartWork{}, ErrUploadPartNotFound
	}
	work := s.works[s.next]
	s.next++
	return work, nil
}

func (s *sequenceWorkStore) MarkStagedPartUploaded(_ context.Context, params MarkStagedPartUploadedParams) (UploadPart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, work := range s.works {
		if work.Part.ID == params.PartID {
			part := work.Part
			part.Status = StatusComplete
			part.TelegramPeer = sql.NullString{String: params.TelegramPeer, Valid: params.TelegramPeer != ""}
			part.MessageID = sql.NullInt64{Int64: params.MessageID, Valid: params.MessageID != 0}
			return part, nil
		}
	}
	return UploadPart{}, ErrUploadPartNotFound
}

func (s *sequenceWorkStore) CompleteUpload(_ context.Context, params CompleteUploadParams) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed = append(s.completed, params)
	return File{ID: params.UploadID + "-file"}, nil
}

func (s *sequenceWorkStore) MarkLocalStagingDeleted(context.Context, string) error { return nil }

func (s *sequenceWorkStore) RetryQueuedPart(context.Context, RetryPartParams) error { return nil }

func (s *sequenceWorkStore) FailQueuedPart(context.Context, string, error) error { return nil }

func testQueuedWork(partID string, uploadID string, storageKey string, encryptedSession []byte, maxParallelUploads int) QueuedPartWork {
	return QueuedPartWork{
		Part: UploadPart{
			ID:             partID,
			UploadID:       uploadID,
			PartNumber:     1,
			PlaintextSize:  sql.NullInt64{Int64: 4 * 1024 * 1024, Valid: true},
			StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
			StorageKey:     sql.NullString{String: storageKey, Valid: true},
		},
		OwnerID:            "owner-1",
		OwnerTelegramID:    12345,
		EncryptedSession:   encryptedSession,
		StoragePeer:        sql.NullString{String: "self", Valid: true},
		MaxParallelUploads: maxParallelUploads,
	}
}

type gateWorkerTelegram struct {
	mu          sync.Mutex
	active      int
	max         int
	uploads     int
	started     chan struct{}
	releaseC    chan struct{}
	done        chan struct{}
	releaseOnce sync.Once
	completed   int
	needDone    int
	startOnce   sync.Once
}

func newGateWorkerTelegram(needDone int) *gateWorkerTelegram {
	return &gateWorkerTelegram{
		started:  make(chan struct{}),
		releaseC: make(chan struct{}),
		done:     make(chan struct{}),
		needDone: needDone,
	}
}

func (t *gateWorkerTelegram) UploadEncryptedPart(_ context.Context, session string, storagePeer string, name string, mimeType string, body io.Reader) (auth.TelegramUploadResult, error) {
	if _, err := io.ReadAll(body); err != nil {
		return auth.TelegramUploadResult{}, err
	}

	t.mu.Lock()
	t.active++
	t.uploads++
	if t.active > t.max {
		t.max = t.active
	}
	if t.active == 2 {
		t.startOnce.Do(func() { close(t.started) })
	}
	t.mu.Unlock()

	<-t.releaseC

	t.mu.Lock()
	t.active--
	t.completed++
	if t.completed >= t.needDone {
		select {
		case <-t.done:
		default:
			close(t.done)
		}
	}
	t.mu.Unlock()

	return auth.TelegramUploadResult{Peer: "self", MessageID: 100}, nil
}

func (t *gateWorkerTelegram) DownloadEncryptedPart(context.Context, string, string, int64, io.Writer) error {
	return nil
}

func (t *gateWorkerTelegram) DeleteEncryptedPart(context.Context, string, string, int64) error {
	return nil
}

func (t *gateWorkerTelegram) release() {
	t.releaseOnce.Do(func() { close(t.releaseC) })
	<-t.done
}

func (t *gateWorkerTelegram) maxActive() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.max
}

func (t *gateWorkerTelegram) uploadCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.uploads
}
