package uploads

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/fnv"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/crypto/agefile"
	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/telegramartifact"
)

const (
	uploadTTL = 24 * time.Hour
)

type Settings struct {
	PartSize                  int64
	StagingDir                string
	EffectiveSettingsProvider func(context.Context, string) (EffectiveSettings, error)
}

type EffectiveSettings struct {
	PartSize                     int64
	MaxParallelUploads           int
	TargetUploadBytesPerSecond   int64
	CooldownBetweenPartsMillisec int
}

type Handler struct {
	store         *Store
	ageRecipient  age.Recipient
	sessionCrypto auth.TelegramSessionCrypto
	telegram      auth.TelegramStorageClient
	settings      Settings
	staging       *LocalSpool
	now           func() time.Time
}

func NewHandler(db *sql.DB, ageRecipient age.Recipient, sessionCrypto auth.TelegramSessionCrypto, telegram auth.TelegramStorageClient, settings Settings) *Handler {
	var staging *LocalSpool
	if strings.TrimSpace(settings.StagingDir) != "" {
		spool, err := NewLocalSpool(settings.StagingDir)
		if err != nil {
			panic("upload staging initialization failed: " + err.Error())
		}
		staging = spool
	}
	return &Handler{
		store:         NewStore(db),
		ageRecipient:  ageRecipient,
		sessionCrypto: sessionCrypto,
		telegram:      telegram,
		settings:      settings,
		staging:       staging,
		now:           time.Now,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	var request createUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	name := normalizeName(request.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "upload_name_required")
		return
	}
	if len(name) > 255 {
		writeError(w, http.StatusBadRequest, "upload_name_too_long")
		return
	}
	if request.PlaintextSize < 0 {
		writeError(w, http.StatusBadRequest, "plaintext_size_invalid")
		return
	}

	checksumAlgorithm, checksum, err := parseChecksum(request.Checksum)
	if err != nil {
		writeError(w, http.StatusBadRequest, "checksum_invalid")
		return
	}

	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	}
	if len(idempotencyKey) > 255 {
		writeError(w, http.StatusBadRequest, "idempotency_key_too_long")
		return
	}

	effectiveSettings, err := h.effectiveSettings(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_settings_load_failed")
		return
	}

	upload, err := h.store.Create(r.Context(), CreateUploadParams{
		OwnerID:           user.ID,
		ParentID:          strings.TrimSpace(request.ParentID),
		Name:              name,
		MimeType:          strings.TrimSpace(request.MimeType),
		PlaintextSize:     request.PlaintextSize,
		PartSize:          effectiveSettings.PartSize,
		IdempotencyKey:    idempotencyKey,
		ChecksumAlgorithm: checksumAlgorithm,
		Checksum:          checksum,
		ExpiresAt:         h.now().Add(uploadTTL),
	})
	if errors.Is(err, ErrParentNotFound) {
		writeError(w, http.StatusNotFound, "parent_folder_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_create_failed")
		return
	}
	_, parts, err := h.store.GetWithParts(r.Context(), user.ID, upload.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_parts_load_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"upload": uploadResponse(upload, parts),
	})
}

func (h *Handler) effectiveSettings(ctx context.Context, userID string) (EffectiveSettings, error) {
	if h.settings.EffectiveSettingsProvider != nil {
		return h.settings.EffectiveSettingsProvider(ctx, userID)
	}
	return EffectiveSettings{PartSize: h.settings.PartSize, MaxParallelUploads: 1}, nil
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	uploadID := strings.TrimSpace(r.PathValue("id"))
	if uploadID == "" {
		writeError(w, http.StatusBadRequest, "upload_id_required")
		return
	}

	upload, parts, err := h.store.GetWithParts(r.Context(), user.ID, uploadID)
	if errors.Is(err, ErrUploadNotFound) {
		writeError(w, http.StatusNotFound, "upload_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_get_failed")
		return
	}

	effectiveSettings, err := h.effectiveSettings(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_settings_load_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload":   uploadResponse(upload, parts),
		"parts":    uploadPartsResponse(parts),
		"progress": uploadProgressResponse(upload, parts, effectiveSettings, h.now),
	})
}

func (h *Handler) UploadPart(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	uploadID := strings.TrimSpace(r.PathValue("id"))
	if uploadID == "" {
		writeError(w, http.StatusBadRequest, "upload_id_required")
		return
	}

	partNumber, err := strconv.Atoi(strings.TrimSpace(r.PathValue("part_number")))
	if err != nil || partNumber < 1 {
		writeError(w, http.StatusBadRequest, "part_number_invalid")
		return
	}

	now := h.now()
	upload, err := h.store.UploadIntegrityState(r.Context(), user.ID, uploadID, partNumber, now)
	if errors.Is(err, ErrUploadNotFound) {
		writeError(w, http.StatusNotFound, "upload_not_found")
		return
	} else if errors.Is(err, ErrUploadExpired) {
		writeError(w, http.StatusConflict, "upload_expired")
		return
	} else if errors.Is(err, ErrUploadPartOutOfOrder) {
		writeError(w, http.StatusConflict, "upload_part_out_of_order")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_check_failed")
		return
	}

	if h.staging != nil {
		h.stageUploadPart(w, r, user.ID, uploadID, partNumber, upload, now)
		return
	}

	telegramSession, err := h.store.TelegramSession(r.Context(), user.ID)
	if errors.Is(err, ErrTelegramSessionNotFound) {
		writeError(w, http.StatusConflict, "telegram_session_missing")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_session_load_failed")
		return
	}

	session, err := h.sessionCrypto.DecryptForTelegramID(user.TelegramID, telegramSession.EncryptedSession)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_session_decrypt_failed")
		return
	}

	peer := nullableString(telegramSession.StoragePeer)
	artifactSpec := telegramartifact.SpecForArtifactID(uploadPartArtifactID(uploadID, partNumber))
	artifactName := artifactSpec.Name()
	plaintextHash, err := agefile.NewSHA256FromState(upload.ChecksumState)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_checksum_state_invalid")
		return
	}

	reader, writer := io.Pipe()
	resultCh := make(chan encryptResult, 1)
	go func() {
		result, err := agefile.EncryptStreamWithHash(writer, r.Body, h.ageRecipient, plaintextHash)
		if err != nil {
			_ = writer.CloseWithError(err)
			resultCh <- encryptResult{err: err}
			return
		}
		_ = writer.Close()
		resultCh <- encryptResult{result: result}
	}()
	wrappedReader := telegramartifact.WrapReader(uploadPartArtifactID(uploadID, partNumber), reader)

	telegramResult, err := h.telegram.UploadEncryptedPart(r.Context(), session, peer, artifactName, artifactSpec.MIMEType(), wrappedReader)
	if err != nil {
		_ = reader.CloseWithError(err)
		_ = h.store.MarkPartFailed(r.Context(), user.ID, uploadID, partNumber)
		writeError(w, http.StatusBadGateway, "telegram_part_upload_failed")
		return
	}

	encrypted := <-resultCh
	if encrypted.err != nil {
		writeError(w, http.StatusInternalServerError, "part_encrypt_failed")
		return
	}

	result := encrypted.result

	part, err := h.store.CompletePart(r.Context(), CompletePartParams{
		OwnerID:        user.ID,
		UploadID:       uploadID,
		PartNumber:     partNumber,
		PlaintextSize:  result.PlaintextSize,
		CiphertextSize: result.CiphertextSize,
		Checksum:       result.Checksum,
		ChecksumState:  result.HashState,
		UploadedSize:   upload.UploadedSize + result.PlaintextSize,
		TelegramPeer:   telegramResult.Peer,
		MessageID:      telegramResult.MessageID,
		Now:            now,
	})
	if errors.Is(err, ErrUploadNotFound) {
		writeError(w, http.StatusNotFound, "upload_not_found")
		return
	}
	if errors.Is(err, ErrUploadExpired) {
		writeError(w, http.StatusConflict, "upload_expired")
		return
	}
	if errors.Is(err, ErrUploadPartOutOfOrder) {
		writeError(w, http.StatusConflict, "upload_part_out_of_order")
		return
	}
	if errors.Is(err, ErrUploadPartSizeMismatch) {
		writeError(w, http.StatusConflict, "upload_part_size_mismatch")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "part_store_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"part": uploadPartResponse(part),
	})
}

func (h *Handler) stageUploadPart(w http.ResponseWriter, r *http.Request, ownerID string, uploadID string, partNumber int, upload Upload, now time.Time) {
	plaintextHash, err := agefile.NewSHA256FromState(upload.ChecksumState)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_checksum_state_invalid")
		return
	}

	storageKey := stagedPartKey(uploadID, partNumber)
	var result agefile.EncryptResult
	if err := h.staging.Write(r.Context(), storageKey, func(writer io.Writer) error {
		encrypted, err := agefile.EncryptStreamWithHash(writer, r.Body, h.ageRecipient, plaintextHash)
		if err != nil {
			return err
		}
		result = encrypted
		return nil
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "part_stage_failed")
		return
	}

	part, err := h.store.StagePart(r.Context(), StagePartParams{
		OwnerID:        ownerID,
		UploadID:       uploadID,
		PartNumber:     partNumber,
		PlaintextSize:  result.PlaintextSize,
		CiphertextSize: result.CiphertextSize,
		Checksum:       result.Checksum,
		ChecksumState:  result.HashState,
		UploadedSize:   upload.UploadedSize + result.PlaintextSize,
		StorageBackend: LocalStagingBackend,
		StorageKey:     storageKey,
		AvailableAt:    now,
		Now:            now,
	})
	if errors.Is(err, ErrUploadNotFound) {
		_ = h.staging.Delete(storageKey)
		writeError(w, http.StatusNotFound, "upload_not_found")
		return
	}
	if errors.Is(err, ErrUploadExpired) {
		_ = h.staging.Delete(storageKey)
		writeError(w, http.StatusConflict, "upload_expired")
		return
	}
	if errors.Is(err, ErrUploadPartOutOfOrder) {
		_ = h.staging.Delete(storageKey)
		writeError(w, http.StatusConflict, "upload_part_out_of_order")
		return
	}
	if errors.Is(err, ErrUploadPartSizeMismatch) {
		_ = h.staging.Delete(storageKey)
		writeError(w, http.StatusConflict, "upload_part_size_mismatch")
		return
	}
	if err != nil {
		_ = h.staging.Delete(storageKey)
		writeError(w, http.StatusInternalServerError, "part_stage_store_failed")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"part": uploadPartResponse(part),
		"queue": map[string]any{
			"status":      "queued",
			"storage_key": storageKey,
			"name":        telegramArtifactName(part.ID),
		},
	})
}

func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_authenticated_user")
		return
	}

	uploadID := strings.TrimSpace(r.PathValue("id"))
	if uploadID == "" {
		writeError(w, http.StatusBadRequest, "upload_id_required")
		return
	}

	file, err := h.store.CompleteUpload(r.Context(), CompleteUploadParams{
		OwnerID:  user.ID,
		UploadID: uploadID,
		Now:      h.now(),
	})
	if errors.Is(err, ErrUploadNotFound) {
		writeError(w, http.StatusNotFound, "upload_not_found")
		return
	}
	if errors.Is(err, ErrUploadExpired) {
		writeError(w, http.StatusConflict, "upload_expired")
		return
	}
	if errors.Is(err, ErrUploadIncomplete) {
		writeError(w, http.StatusConflict, "upload_incomplete")
		return
	}
	if errors.Is(err, ErrUploadSizeMismatch) {
		writeError(w, http.StatusConflict, "upload_size_mismatch")
		return
	}
	if errors.Is(err, ErrUploadChecksumMismatch) {
		writeError(w, http.StatusConflict, "upload_checksum_mismatch")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_complete_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"file": fileResponse(file),
	})
}

type createUploadRequest struct {
	Name           string `json:"name"`
	ParentID       string `json:"parent_id"`
	MimeType       string `json:"mime_type"`
	PlaintextSize  int64  `json:"plaintext_size"`
	Checksum       string `json:"checksum"`
	IdempotencyKey string `json:"idempotency_key"`
}

func normalizeName(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "/", ""))
}

func parseChecksum(value string) (string, []byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil, nil
	}

	checksum, err := hex.DecodeString(value)
	if err != nil || len(checksum) != 32 {
		return "", nil, errors.New("invalid sha256 checksum")
	}

	return "sha256", checksum, nil
}

func uploadResponse(upload Upload, parts []UploadPart) map[string]any {
	plan := uploadPartPlanFromParts(parts)
	if len(plan) == 0 {
		plan = uploadPartPlan(upload.ID, nullableInt64(upload.PlaintextSize), upload.PartSize)
	}
	return map[string]any{
		"id":                 upload.ID,
		"parent_id":          nullableStringValue(upload.ParentID),
		"name":               upload.NamePlain,
		"mime_type":          nullableStringValue(upload.MimeType),
		"plaintext_size":     nullableInt64Value(upload.PlaintextSize),
		"part_size":          upload.PartSize,
		"part_count":         len(plan),
		"part_plan":          uploadPartPlanResponse(plan),
		"status":             upload.Status,
		"idempotency_key":    nullableStringValue(upload.IdempotencyKey),
		"checksum_algorithm": nullableStringValue(upload.ChecksumAlgorithm),
		"uploaded_size":      upload.UploadedSize,
		"next_part_number":   upload.NextPartNumber,
		"created_at":         upload.CreatedAt,
		"updated_at":         upload.UpdatedAt,
		"expires_at":         upload.ExpiresAt,
	}
}

type encryptResult struct {
	result agefile.EncryptResult
	err    error
}

func uploadPartResponse(part UploadPart) map[string]any {
	return map[string]any{
		"id":                  part.ID,
		"upload_id":           part.UploadID,
		"part_number":         part.PartNumber,
		"plaintext_start":     nullableInt64Value(part.PlaintextStart),
		"plaintext_end":       nullableInt64Value(part.PlaintextEnd),
		"plaintext_size":      nullableInt64Value(part.PlaintextSize),
		"ciphertext_size":     nullableInt64Value(part.CiphertextSize),
		"checksum":            hex.EncodeToString(part.Checksum),
		"telegram_peer":       nullableStringValue(part.TelegramPeer),
		"telegram_message_id": nullableInt64Value(part.MessageID),
		"status":              part.Status,
		"storage_backend":     nullableStringValue(part.StorageBackend),
		"storage_key":         nullableStringValue(part.StorageKey),
		"available_at":        part.AvailableAt,
		"leased_until":        nullableTimeValue(part.LeasedUntil),
		"attempts":            part.Attempts,
		"last_error":          nullableStringValue(part.LastError),
		"worker_id":           nullableStringValue(part.WorkerID),
		"created_at":          part.CreatedAt,
		"updated_at":          part.UpdatedAt,
	}
}

func uploadPartsResponse(parts []UploadPart) []map[string]any {
	out := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		out = append(out, uploadPartResponse(part))
	}
	return out
}

func uploadProgressResponse(upload Upload, parts []UploadPart, settings EffectiveSettings, now func() time.Time) map[string]any {
	expectedParts := int64(len(parts))
	if expectedParts == 0 {
		expectedParts = int64(len(uploadPartPlan(upload.ID, nullableInt64(upload.PlaintextSize), upload.PartSize)))
	}
	receivedParts := 0
	progress := map[string]any{
		"expected_parts":           expectedParts,
		"received_parts":           0,
		"queued_parts":             0,
		"leased_parts":             0,
		"complete_parts":           0,
		"failed_parts":             0,
		"plaintext_received_size":  upload.UploadedSize,
		"plaintext_complete_size":  int64(0),
		"ciphertext_staged_size":   int64(0),
		"ciphertext_complete_size": int64(0),
		"next_retry_at":            nil,
		"active_workers":           []string{},
		"ready_to_complete":        false,
		"upload_policy":            uploadPolicyResponse(settings),
	}

	activeWorkers := make(map[string]struct{})
	var nextRetry sql.NullTime
	currentTime := now()
	for _, part := range parts {
		if part.CiphertextSize.Valid || len(part.Checksum) > 0 || part.StorageKey.Valid || part.TelegramPeer.Valid || part.MessageID.Valid || part.Status == StatusComplete || part.Status == "failed" {
			receivedParts++
		}
		if part.PlaintextSize.Valid && part.Status == StatusComplete {
			progress["plaintext_complete_size"] = progress["plaintext_complete_size"].(int64) + part.PlaintextSize.Int64
		}
		if part.CiphertextSize.Valid {
			if part.StorageKey.Valid && part.Status != StatusComplete {
				progress["ciphertext_staged_size"] = progress["ciphertext_staged_size"].(int64) + part.CiphertextSize.Int64
			}
			if part.Status == StatusComplete {
				progress["ciphertext_complete_size"] = progress["ciphertext_complete_size"].(int64) + part.CiphertextSize.Int64
			}
		}

		switch part.Status {
		case StatusComplete:
			progress["complete_parts"] = progress["complete_parts"].(int) + 1
		case "failed":
			progress["failed_parts"] = progress["failed_parts"].(int) + 1
		default:
			if part.LeasedUntil.Valid && part.LeasedUntil.Time.After(currentTime) {
				progress["leased_parts"] = progress["leased_parts"].(int) + 1
				if part.WorkerID.Valid {
					activeWorkers[part.WorkerID.String] = struct{}{}
				}
			} else {
				progress["queued_parts"] = progress["queued_parts"].(int) + 1
			}
			if part.AvailableAt.After(currentTime) && (!nextRetry.Valid || part.AvailableAt.Before(nextRetry.Time)) {
				nextRetry = sql.NullTime{Time: part.AvailableAt, Valid: true}
			}
		}
	}
	progress["received_parts"] = receivedParts

	if nextRetry.Valid {
		progress["next_retry_at"] = nextRetry.Time
	}
	workers := make([]string, 0, len(activeWorkers))
	for workerID := range activeWorkers {
		workers = append(workers, workerID)
	}
	sort.Strings(workers)
	progress["active_workers"] = workers
	progress["ready_to_complete"] = upload.Status != StatusComplete &&
		expectedParts > 0 &&
		int64(progress["complete_parts"].(int)) == expectedParts &&
		progress["failed_parts"].(int) == 0
	return progress
}

func uploadPartPlanFromParts(parts []UploadPart) []UploadPartRange {
	if len(parts) == 0 {
		return nil
	}
	plan := make([]UploadPartRange, 0, len(parts))
	for _, part := range parts {
		if !part.PlaintextStart.Valid || !part.PlaintextEnd.Valid || !part.PlaintextSize.Valid {
			return nil
		}
		plan = append(plan, UploadPartRange{
			PartNumber: part.PartNumber,
			Start:      part.PlaintextStart.Int64,
			End:        part.PlaintextEnd.Int64,
			Size:       part.PlaintextSize.Int64,
		})
	}
	sort.Slice(plan, func(i, j int) bool {
		return plan[i].PartNumber < plan[j].PartNumber
	})
	return plan
}

func uploadPolicyResponse(settings EffectiveSettings) map[string]any {
	maxParallelUploads := settings.MaxParallelUploads
	if maxParallelUploads <= 0 {
		maxParallelUploads = 1
	}
	targetUploadBytesPerSecond := settings.TargetUploadBytesPerSecond
	if targetUploadBytesPerSecond < 0 {
		targetUploadBytesPerSecond = 0
	}
	cooldownBetweenPartsMillisec := settings.CooldownBetweenPartsMillisec
	if cooldownBetweenPartsMillisec < 0 {
		cooldownBetweenPartsMillisec = 0
	}
	return map[string]any{
		"max_parallel_uploads":           maxParallelUploads,
		"target_upload_bytes_per_second": targetUploadBytesPerSecond,
		"cooldown_between_parts_ms":      cooldownBetweenPartsMillisec,
	}
}

func fileResponse(file File) map[string]any {
	return map[string]any{
		"id":              file.ID,
		"parent_id":       nullableStringValue(file.ParentID),
		"name":            nullableStringValue(file.NamePlain),
		"mime_type":       nullableStringValue(file.MimeType),
		"plaintext_size":  nullableInt64Value(file.PlaintextSize),
		"ciphertext_size": nullableInt64Value(file.CiphertextSize),
		"type":            file.Type,
		"status":          file.Status,
		"created_at":      file.CreatedAt,
		"updated_at":      file.UpdatedAt,
	}
}

func uploadPartName(uploadID string, partNumber int) string {
	return uploadID + ".part-" + strconv.Itoa(partNumber) + ".age"
}

type UploadPartRange struct {
	PartNumber int
	Start      int64
	End        int64
	Size       int64
}

func uploadPartPlan(uploadID string, size int64, maxPartSize int64) []UploadPartRange {
	if size <= 0 || maxPartSize <= 0 {
		return nil
	}
	count := int(partCount(size, maxPartSize))
	if count <= 1 {
		return []UploadPartRange{{PartNumber: 1, Start: 0, End: size, Size: size}}
	}

	minPartSize := maxPartSize / 2
	if minPartSize < 8*1024*1024 {
		minPartSize = minInt64(maxPartSize, 8*1024*1024)
	}

	plan := make([]UploadPartRange, 0, count)
	var start int64
	for part := 1; part <= count; part++ {
		remaining := size - start
		remainingParts := int64(count - part + 1)
		partSize := remaining
		if remainingParts > 1 {
			low := maxInt64(minPartSize, remaining-maxPartSize*(remainingParts-1))
			high := minInt64(maxPartSize, remaining-minPartSize*(remainingParts-1))
			if high < low {
				low = maxInt64(1, remaining/remainingParts)
				high = minInt64(maxPartSize, remaining-(remainingParts-1))
			}
			partSize = stableRange(uploadID, part, low, high)
		}
		end := start + partSize
		plan = append(plan, UploadPartRange{PartNumber: part, Start: start, End: end, Size: partSize})
		start = end
	}
	return plan
}

func uploadPartPlanResponse(plan []UploadPartRange) []map[string]any {
	out := make([]map[string]any, 0, len(plan))
	for _, part := range plan {
		out = append(out, map[string]any{
			"part_number": part.PartNumber,
			"start":       part.Start,
			"end":         part.End,
			"size":        part.Size,
		})
	}
	return out
}

func stableRange(uploadID string, part int, low int64, high int64) int64 {
	if high <= low {
		return low
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(uploadID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strconv.Itoa(part)))
	span := uint64(high - low + 1)
	return low + int64(h.Sum64()%span)
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func uploadPartArtifactID(uploadID string, partNumber int) string {
	return uploadID + "-part-" + strconv.Itoa(partNumber)
}

func telegramArtifactName(artifactID string) string {
	return telegramartifact.SpecForArtifactID(artifactID).Name()
}

func telegramArtifactMimeType(artifactID string) string {
	return telegramartifact.SpecForArtifactID(artifactID).MIMEType()
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullableStringValue(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullableInt64Value(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullableTimeValue(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{
		"error": code,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
