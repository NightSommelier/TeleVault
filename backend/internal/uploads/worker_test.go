package uploads

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/televault/TeleVault/backend/internal/auth"
	"github.com/televault/TeleVault/backend/internal/crypto/secrets"
)

func TestDrainWorkerUploadsStagedPartAndDeletesLocalCopy(t *testing.T) {
	ctx := context.Background()
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
				StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
				StorageKey:     sql.NullString{String: "upload-1/part-1.age", Valid: true},
				Attempts:       1,
			},
			OwnerTelegramID:  12345,
			EncryptedSession: encryptedSession,
			StoragePeer:      sql.NullString{String: "self", Valid: true},
			UploadName:       "photo.png",
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
	if telegram.session != "telegram-session" || telegram.peer != "self" || telegram.name != "photo.png.part-1.age" || string(telegram.body) != "ciphertext" {
		t.Fatalf("telegram upload = session %q peer %q name %q body %q", telegram.session, telegram.peer, telegram.name, string(telegram.body))
	}
	if store.marked.PartID != "part-1" || store.marked.TelegramPeer != "self" || store.marked.MessageID != 77 {
		t.Fatalf("MarkStagedPartUploaded() params = %+v", store.marked)
	}
	if _, err := spool.Open("upload-1/part-1.age"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("spool.Open() after drain error = %v, want os.ErrNotExist", err)
	}
}

func TestDrainWorkerRetriesFloodWait(t *testing.T) {
	ctx := context.Background()
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

func testSessionCrypto(t *testing.T) auth.TelegramSessionCrypto {
	t.Helper()
	box, err := secrets.NewBox(bytes.Repeat([]byte{1}, secrets.KeyBytes))
	if err != nil {
		t.Fatalf("NewBox() error = %v", err)
	}
	return auth.NewTelegramSessionCrypto(box)
}

type fakeWorkStore struct {
	work      QueuedPartWork
	claimed   bool
	marked    MarkStagedPartUploadedParams
	retry     RetryPartParams
	failedID  string
	failedErr error
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
	body      []byte
}

func (t *fakeWorkerTelegram) UploadEncryptedPart(_ context.Context, session string, storagePeer string, name string, body io.Reader) (auth.TelegramUploadResult, error) {
	t.session = session
	t.peer = storagePeer
	t.name = name
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
