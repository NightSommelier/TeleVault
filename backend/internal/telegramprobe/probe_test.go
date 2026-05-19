package telegramprobe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
)

func TestDryRunBuildsProbeSizes(t *testing.T) {
	result, err := DryRun(Plan{MinBytes: 1, MaxBytes: 5, StepBytes: 2})
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}
	if !result.DryRun {
		t.Fatal("DryRun() DryRun = false, want true")
	}
	want := []int64{1, 3, 5}
	if !sameInt64s(result.AttemptedSizes, want) {
		t.Fatalf("AttemptedSizes = %v, want %v", result.AttemptedSizes, want)
	}
}

func TestRunUploadsAndDeletesEachProbeFile(t *testing.T) {
	client := &fakeProbeClient{}
	result, err := Run(context.Background(), client, "session", "self", Plan{MinBytes: 2, MaxBytes: 4, StepBytes: 2})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.DetectedBytes != 4 {
		t.Fatalf("DetectedBytes = %d, want 4", result.DetectedBytes)
	}
	if !sameInt64s(client.uploadedSizes, []int64{2, 4}) {
		t.Fatalf("uploaded sizes = %v, want [2 4]", client.uploadedSizes)
	}
	if !sameInt64s(client.deletedMessageIDs, []int64{101, 102}) {
		t.Fatalf("deleted message IDs = %v, want [101 102]", client.deletedMessageIDs)
	}
}

func TestRunStopsOnFirstFailedUpload(t *testing.T) {
	client := &fakeProbeClient{failAtSize: 4}
	result, err := Run(context.Background(), client, "session", "self", Plan{MinBytes: 2, MaxBytes: 6, StepBytes: 2})
	if !errors.Is(err, errFakeUpload) {
		t.Fatalf("Run() error = %v, want errFakeUpload", err)
	}
	if result.DetectedBytes != 2 || result.FailedBytes != 4 {
		t.Fatalf("result = %+v, want detected 2 and failed 4", result)
	}
	if !sameInt64s(client.deletedMessageIDs, []int64{101}) {
		t.Fatalf("deleted message IDs = %v, want [101]", client.deletedMessageIDs)
	}
}

var errFakeUpload = errors.New("fake upload failed")

type fakeProbeClient struct {
	failAtSize        int64
	uploadedSizes     []int64
	deletedMessageIDs []int64
	nextMessageID     int64
}

func (c *fakeProbeClient) UploadEncryptedPart(ctx context.Context, session string, storagePeer string, name string, mimeType string, body io.Reader) (auth.TelegramUploadResult, error) {
	var buf bytes.Buffer
	size, err := io.Copy(&buf, body)
	if err != nil {
		return auth.TelegramUploadResult{}, err
	}
	if size == c.failAtSize {
		return auth.TelegramUploadResult{}, errFakeUpload
	}
	c.uploadedSizes = append(c.uploadedSizes, size)
	c.nextMessageID++
	return auth.TelegramUploadResult{Peer: storagePeer, MessageID: 100 + c.nextMessageID}, nil
}

func (c *fakeProbeClient) DeleteEncryptedPart(ctx context.Context, session string, storagePeer string, messageID int64) error {
	c.deletedMessageIDs = append(c.deletedMessageIDs, messageID)
	return nil
}

func sameInt64s(a []int64, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
