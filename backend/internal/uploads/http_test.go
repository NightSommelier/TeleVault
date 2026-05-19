package uploads

import (
	"database/sql"
	"testing"
	"time"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/telegramartifact"
)

func TestNormalizeName(t *testing.T) {
	tests := map[string]string{
		" Report.pdf ": "Report.pdf",
		"/bad/name/":   "badname",
		"   ":          "",
	}

	for input, want := range tests {
		if got := normalizeName(input); got != want {
			t.Fatalf("normalizeName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseChecksum(t *testing.T) {
	algorithm, checksum, err := parseChecksum("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("parseChecksum returned error: %v", err)
	}
	if algorithm != "sha256" {
		t.Fatalf("algorithm = %q, want sha256", algorithm)
	}
	if len(checksum) != 32 {
		t.Fatalf("checksum length = %d, want 32", len(checksum))
	}
}

func TestParseChecksumRejectsInvalidValue(t *testing.T) {
	if _, _, err := parseChecksum("abc"); err == nil {
		t.Fatal("parseChecksum accepted invalid checksum")
	}
}

func TestPartCount(t *testing.T) {
	partSize := int64(64 * 1024 * 1024)
	tests := map[int64]int64{
		0:            0,
		1:            1,
		partSize:     1,
		partSize + 1: 2,
	}

	for size, want := range tests {
		if got := partCount(size, partSize); got != want {
			t.Fatalf("partCount(%d) = %d, want %d", size, got, want)
		}
	}
}

func TestUploadPartResponseIncludesChecksumHex(t *testing.T) {
	part := UploadPart{
		Checksum: []byte{0xde, 0xad, 0xbe, 0xef},
	}

	response := uploadPartResponse(part)
	if response["checksum"] != "deadbeef" {
		t.Fatalf("checksum = %v, want deadbeef", response["checksum"])
	}
}

func TestTelegramArtifactNameDoesNotUseOriginalFilename(t *testing.T) {
	spec := telegramartifact.SpecForArtifactID("6c851bbf-57f2-46ea-a216-ee029365750f")
	if got := telegramArtifactName(spec.ArtifactID); got != spec.Name() {
		t.Fatalf("telegramArtifactName() = %q, want %q", got, spec.Name())
	}
	if got := telegramArtifactMimeType(spec.ArtifactID); got != spec.MIMEType() {
		t.Fatalf("telegramArtifactMimeType() = %q, want %q", got, spec.MIMEType())
	}
	if got := uploadPartArtifactID("upload-1", 2); got != "upload-1-part-2" {
		t.Fatalf("uploadPartArtifactID() = %q", got)
	}
}

func TestUploadProgressResponseSummarizesQueueState(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	upload := Upload{
		PlaintextSize: sql.NullInt64{Int64: 30, Valid: true},
		PartSize:      10,
		UploadedSize:  20,
	}
	parts := []UploadPart{
		{
			PartNumber:     1,
			Status:         StatusComplete,
			PlaintextSize:  sql.NullInt64{Int64: 10, Valid: true},
			CiphertextSize: sql.NullInt64{Int64: 18, Valid: true},
			StorageKey:     sql.NullString{String: "upload/part-1.age", Valid: true},
		},
		{
			PartNumber:     2,
			Status:         StatusPending,
			CiphertextSize: sql.NullInt64{Int64: 17, Valid: true},
			StorageKey:     sql.NullString{String: "upload/part-2.age", Valid: true},
			LeasedUntil:    sql.NullTime{Time: now.Add(time.Minute), Valid: true},
			WorkerID:       sql.NullString{String: "worker-b", Valid: true},
		},
		{
			PartNumber:  3,
			Status:      StatusPending,
			AvailableAt: now.Add(30 * time.Second),
		},
	}

	settings := EffectiveSettings{
		MaxParallelUploads:           3,
		TargetUploadBytesPerSecond:   10_000_000,
		CooldownBetweenPartsMillisec: 500,
	}
	progress := uploadProgressResponse(upload, parts, settings, func() time.Time { return now })
	if progress["expected_parts"] != int64(3) ||
		progress["received_parts"] != 3 ||
		progress["queued_parts"] != 1 ||
		progress["leased_parts"] != 1 ||
		progress["complete_parts"] != 1 ||
		progress["failed_parts"] != 0 ||
		progress["plaintext_received_size"] != int64(20) ||
		progress["plaintext_complete_size"] != int64(10) ||
		progress["ciphertext_staged_size"] != int64(17) ||
		progress["ciphertext_complete_size"] != int64(18) ||
		progress["ready_to_complete"] != false {
		t.Fatalf("uploadProgressResponse() = %+v", progress)
	}
	if progress["next_retry_at"] != now.Add(30*time.Second) {
		t.Fatalf("next_retry_at = %v, want %v", progress["next_retry_at"], now.Add(30*time.Second))
	}
	workers, ok := progress["active_workers"].([]string)
	if !ok || len(workers) != 1 || workers[0] != "worker-b" {
		t.Fatalf("active_workers = %#v, want [worker-b]", progress["active_workers"])
	}
	policy, ok := progress["upload_policy"].(map[string]any)
	if !ok {
		t.Fatalf("upload_policy = %#v, want object", progress["upload_policy"])
	}
	if policy["max_parallel_uploads"] != 3 ||
		policy["target_upload_bytes_per_second"] != int64(10_000_000) ||
		policy["cooldown_between_parts_ms"] != 500 {
		t.Fatalf("upload_policy = %#v", policy)
	}
	if _, ok := policy["telegram_peer"]; ok {
		t.Fatalf("upload_policy exposes telegram_peer: %#v", policy)
	}
}
