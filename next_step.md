# Completed: Drag-and-Drop Folder Move and Main-Area Upload Ergonomics

Status: completed on 2026-05-19.

The previous task tightened the embedded web UI drag-and-drop behavior in `backend/internal/httpserver/static/index.html`:

- dropping external files on the main file area now enqueues uploads immediately for the current folder;
- dropping external files on the sidebar dropzone still stages the selection for explicit confirmation;
- dragging an existing file/folder onto another folder still moves it into that folder;
- dragging an existing file/folder onto breadcrumbs or the `Up` control now moves it to an ancestor folder or root.

The docs were updated in `docs/development/web.md`, and the privacy-masking plan was clarified in `TeleVault-plan.md`.

Do not redo this drag-and-drop task unless a regression is found.

---

# Next Step: Privacy Masking Artifact Camouflage Foundation

## Goal

Make Telegram-side uploaded artifacts less recognizable as raw age-encrypted files.

Today the service keeps original user filenames out of Telegram by uploading opaque artifact names such as `<upload_part_id>.bin`, but the encrypted payload can still start with an obvious age header such as:

```text
age-encryption.org/v1
-> X25519 ...
```

Add a foundation for privacy masking so future and optionally current uploads can use decoy artifact names/extensions/mime families and, when enabled, wrap the ciphertext payload so Telegram does not see the age header at byte zero.

## Context

Recent relevant work:

- Worker uploads already use opaque artifact names instead of original filenames.
- Downloads still serve the original user-facing filename through service metadata.
- The user explicitly wants decoy formats such as `.mp3`, `.mp4`, `.avi`, `.m4v`, `.3gp`, `.jpg`, `.jpeg`, and similar ordinary-looking names.
- The user also wants to avoid exposing raw `age-encryption.org/v1` headers to Telegram storage.

Important files:

- `backend/internal/uploads/http.go`
- `backend/internal/uploads/worker.go`
- `backend/internal/uploads/http_test.go`
- `backend/internal/uploads/worker_test.go`
- `backend/internal/crypto/agefile/`
- `docs/development/uploads.md`
- `docs/threat-model.md`
- `TeleVault-plan.md`

## Required Behavior

1. Define a small privacy-masking design before broad implementation.
2. Add a deterministic artifact-name helper that can choose a decoy extension from an allowlist.
3. Keep the real user filename and MIME type only in service metadata.
4. Do not use the original filename in Telegram artifact names.
5. Do not expose raw public-link tokens, Telegram sessions, AGE private keys, Telegram peer ids, or message ids in any user-facing UI.
6. For payload header masking, implement only a safe foundation:
   - either document the wrapper format clearly and add tests without enabling it yet;
   - or add a backward-compatible wrapper reader/writer with a version marker that the download path can unwrap before age decryption.
7. Existing files already uploaded without masking must remain downloadable.
8. Downloads must keep serving the original user filename.

## Suggested Implementation

Start conservatively:

- Add a `PrivacyMaskingMode` or similarly named internal concept, defaulting to current behavior unless config already has a suitable flag.
- Add a helper such as `telegramArtifactName(artifactID string, mode PrivacyMaskingMode)`.
- Use an allowlist of decoy extensions. Keep it small and explicit at first: `.mp3`, `.mp4`, `.avi`, `.m4v`, `.3gp`, `.jpg`, `.jpeg`, `.zip`, `.pdf`.
- Add tests proving artifact names never include the original filename and always remain deterministic for a given artifact id/mode.
- For age header masking, prefer a reversible wrapper around the ciphertext bytes rather than modifying age itself. The wrapper must be clearly versioned and must not break existing unwrapped ciphertext.

## Constraints

- No lossy transformation of ciphertext.
- No change that makes already uploaded files unreadable.
- No original filenames in Telegram artifact names.
- No random extension selection that makes retries non-idempotent.
- No broad storage schema migration unless absolutely necessary.
- No attempt to fake valid media containers in this step unless the reader/writer path is fully specified and tested.

## Docs

Update:

- `docs/development/uploads.md`
- `docs/threat-model.md`
- `TeleVault-plan.md`

Mention exactly what privacy masking does and does not protect against.

## Verification

Run:

```sh
cd backend
go test ./... -count=1
```

Validate the embedded JS syntax if the web UI changes:

```sh
node -e "const fs=require('fs'); const html=fs.readFileSync('backend/internal/httpserver/static/index.html','utf8'); const m=html.match(/<script>([\\s\\S]*)<\\/script>/); new Function(m[1]); console.log('js ok')"
```

Then run:

```sh
git diff --check
```

## Acceptance Criteria

- Telegram artifact naming has a tested privacy-masking foundation.
- Decoy extensions come from an explicit allowlist.
- Artifact names stay deterministic and do not use original user filenames.
- Existing unmasked downloads remain supported.
- The age-header masking path is either clearly documented for the next implementation step or implemented with backward-compatible tests.
- Docs explain the protection and limits.
- Tests pass.
- No secrets or local test files are committed.
