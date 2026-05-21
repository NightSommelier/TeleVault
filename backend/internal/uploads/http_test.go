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
	partSize := int64(384 * 1024 * 1024)
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

func TestUploadPartPlanUsesUnevenRanges(t *testing.T) {
	maxPartSize := int64(384 * 1024 * 1024)
	size := int64(1300 * 1024 * 1024)

	plan := uploadPartPlan("upload-plan-test", size, maxPartSize)
	if len(plan) != 4 {
		t.Fatalf("len(plan) = %d, want 4", len(plan))
	}

	var cursor int64
	sizes := make(map[int64]struct{})
	for index, part := range plan {
		if part.PartNumber != index+1 {
			t.Fatalf("part number at index %d = %d", index, part.PartNumber)
		}
		if part.Start != cursor {
			t.Fatalf("part %d start = %d, want %d", part.PartNumber, part.Start, cursor)
		}
		if part.End <= part.Start {
			t.Fatalf("part %d has invalid range [%d,%d)", part.PartNumber, part.Start, part.End)
		}
		if part.Size != part.End-part.Start {
			t.Fatalf("part %d size = %d, want %d", part.PartNumber, part.Size, part.End-part.Start)
		}
		if part.Size > maxPartSize {
			t.Fatalf("part %d size = %d, exceeds max %d", part.PartNumber, part.Size, maxPartSize)
		}
		sizes[part.Size] = struct{}{}
		cursor = part.End
	}
	if cursor != size {
		t.Fatalf("planned size = %d, want %d", cursor, size)
	}
	if len(sizes) == 1 {
		t.Fatalf("all planned parts have the same size: %+v", plan)
	}
}

func TestUploadResponseIncludesPartPlan(t *testing.T) {
	upload := Upload{
		ID:            "upload-response-plan",
		PlaintextSize: sql.NullInt64{Int64: 100, Valid: true},
		PartSize:      40,
	}
	parts := []UploadPart{
		{PartNumber: 1, PlaintextStart: sql.NullInt64{Int64: 0, Valid: true}, PlaintextEnd: sql.NullInt64{Int64: 31, Valid: true}, PlaintextSize: sql.NullInt64{Int64: 31, Valid: true}},
		{PartNumber: 2, PlaintextStart: sql.NullInt64{Int64: 31, Valid: true}, PlaintextEnd: sql.NullInt64{Int64: 68, Valid: true}, PlaintextSize: sql.NullInt64{Int64: 37, Valid: true}},
		{PartNumber: 3, PlaintextStart: sql.NullInt64{Int64: 68, Valid: true}, PlaintextEnd: sql.NullInt64{Int64: 100, Valid: true}, PlaintextSize: sql.NullInt64{Int64: 32, Valid: true}},
	}

	response := uploadResponse(upload, parts)
	plan, ok := response["part_plan"].([]map[string]any)
	if !ok {
		t.Fatalf("part_plan = %#v, want object array", response["part_plan"])
	}
	if response["part_count"] != len(plan) {
		t.Fatalf("part_count = %#v, want %d", response["part_count"], len(plan))
	}
	if len(plan) != 3 {
		t.Fatalf("len(part_plan) = %d, want 3", len(plan))
	}
	if plan[0]["part_number"] != 1 || plan[0]["start"] != int64(0) {
		t.Fatalf("first part = %#v", plan[0])
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
	const mediumSize = int64(32 * 1024 * 1024)
	spec := telegramartifact.SpecForArtifactIDAndSize("6c851bbf-57f2-46ea-a216-ee029365750f", mediumSize)
	if got := telegramArtifactName(spec.ArtifactID, mediumSize); got != spec.Name() {
		t.Fatalf("telegramArtifactName() = %q, want %q", got, spec.Name())
	}
	if got := telegramArtifactMimeType(spec.ArtifactID, mediumSize); got != spec.MIMEType() {
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
			StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
			StorageKey:     sql.NullString{String: "upload/part-2.age", Valid: true},
			LeasedUntil:    sql.NullTime{Time: now.Add(time.Minute), Valid: true},
			WorkerID:       sql.NullString{String: "worker-b", Valid: true},
		},
		{
			PartNumber:     3,
			Status:         StatusPending,
			PlaintextSize:  sql.NullInt64{Int64: 10, Valid: true},
			StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
			StorageKey:     sql.NullString{String: "upload/part-3.age", Valid: true},
			AvailableAt:    now.Add(30 * time.Second),
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
		progress["telegram_remaining_bytes"] != int64(27) ||
		progress["telegram_eta_seconds"] != int64(2) ||
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

func TestUploadProgressResponseTreatsFailedUploadAsTerminal(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	upload := Upload{
		Status:        "failed",
		PlaintextSize: sql.NullInt64{Int64: 20, Valid: true},
		PartSize:      10,
		UploadedSize:  20,
	}
	parts := []UploadPart{
		{
			PartNumber:    1,
			Status:        StatusPending,
			PlaintextSize: sql.NullInt64{Int64: 10, Valid: true},
		},
		{
			PartNumber:    2,
			Status:        StatusPending,
			PlaintextSize: sql.NullInt64{Int64: 10, Valid: true},
		},
	}

	progress := uploadProgressResponse(upload, parts, EffectiveSettings{}, func() time.Time { return now })
	if progress["failed_parts"] != 2 ||
		progress["queued_parts"] != 0 ||
		progress["leased_parts"] != 0 ||
		progress["telegram_remaining_bytes"] != int64(0) ||
		progress["ready_to_complete"] != false {
		t.Fatalf("uploadProgressResponse() = %+v, want failed terminal progress", progress)
	}
}

func TestUploadProgressResponseEstimatesETAFromCompletedBytes(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	upload := Upload{
		PlaintextSize: sql.NullInt64{Int64: 30, Valid: true},
		PartSize:      10,
		CreatedAt:     now.Add(-10 * time.Second),
	}
	parts := []UploadPart{
		{
			PartNumber:     1,
			Status:         StatusComplete,
			CiphertextSize: sql.NullInt64{Int64: 100, Valid: true},
		},
		{
			PartNumber:     2,
			Status:         StatusPending,
			CiphertextSize: sql.NullInt64{Int64: 50, Valid: true},
			StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
			StorageKey:     sql.NullString{String: "upload/part-2.age", Valid: true},
		},
		{
			PartNumber:     3,
			Status:         StatusPending,
			CiphertextSize: sql.NullInt64{Int64: 50, Valid: true},
			StorageBackend: sql.NullString{String: LocalStagingBackend, Valid: true},
			StorageKey:     sql.NullString{String: "upload/part-3.age", Valid: true},
		},
	}

	progress := uploadProgressResponse(upload, parts, EffectiveSettings{}, func() time.Time { return now })
	if progress["telegram_eta_seconds"] != int64(10) {
		t.Fatalf("telegram_eta_seconds = %v, want 10", progress["telegram_eta_seconds"])
	}
}
