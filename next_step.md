# Completed: File Metadata Details UI Foundation

Status: completed on 2026-05-19.

The previous task added a file and folder details modal in `backend/internal/httpserver/static/index.html`, added file metadata counts to `GET /files/{id}`, updated docs in `docs/development/files.md`, `docs/development/web.md`, and `TeleVault-plan.md`, and passed:

```sh
cd backend
go test ./... -count=1
```

```sh
node -e "const fs=require('fs'); const html=fs.readFileSync('backend/internal/httpserver/static/index.html','utf8'); const m=html.match(/<script>([\\s\\S]*)<\\/script>/); new Function(m[1]); console.log('js ok')"
```

```sh
git diff --check
```

Do not redo the completed details foundation unless a regression is found.

---

# Next Step: File List Metadata Preview and Scannability

## Goal

Make the file table easier to scan at a glance without changing the backend list shape.

The details modal now carries the richer metadata. The next step is to surface a compact metadata preview directly in the file list so users can see ownership and timestamps without opening details first.

## Context

Recent relevant work:

- The embedded web UI already lists files and folders, supports upload/download, delete, move by drag-and-drop, share, public links, and the new details modal.
- `GET /files` and `GET /shared` already return enough fields for a compact list preview: `owner_id`, `created_at`, `updated_at`, `type`, `status`, `plaintext_size`, and `mime_type`.
- The richer counts such as `part_count` and public-link summary stay in the details modal.

Important files:

- `backend/internal/httpserver/static/index.html`
- `docs/development/web.md`
- `TeleVault-plan.md`

## Required Behavior

1. Add a compact metadata preview to each file/folder row in the web UI.
2. The preview should show ownership context and creation time at a glance:
   - `You` for own items;
   - `Shared owner` or equivalent for shared items;
   - created timestamp in a muted, compact form.
3. Keep the row layout stable on desktop and mobile.
4. Do not add new backend list queries or per-row API calls.
5. Keep `Details` as the place for the richer metadata fields.
6. Do not change upload, download, delete, move, share, public-link, or recovery behavior.

## Suggested Implementation

In `backend/internal/httpserver/static/index.html`:

- Add a muted secondary line under the file/folder name or inside the name cell.
- Reuse the fields already present in list responses.
- Keep action buttons and the existing size/status columns intact.
- Keep the UI dependency-free.

## Constraints

- No backend API changes for list endpoints.
- No N+1 queries.
- No broad table redesign.
- No new frontend dependencies.
- No secrets in the UI.

## Docs

Update:

- `docs/development/web.md`
- `TeleVault-plan.md`

Mention that the row preview is intentionally compact and that the richer counters remain in the details modal.

## Verification

Run:

```sh
cd backend
go test ./... -count=1
```

Validate the embedded JS syntax:

```sh
node -e "const fs=require('fs'); const html=fs.readFileSync('backend/internal/httpserver/static/index.html','utf8'); const m=html.match(/<script>([\\s\\S]*)<\\/script>/); new Function(m[1]); console.log('js ok')"
```

Then run:

```sh
git diff --check
```

Manual UI checks if possible:

- Confirm the row preview is visible for own files and shared files.
- Confirm the table stays readable on narrow screens.
- Open the details modal and confirm it still shows the richer metadata.

## Acceptance Criteria

- Each file and folder row shows a compact ownership/timestamp preview.
- The file table stays stable and readable.
- The richer metadata remains available in the details modal.
- Existing file table, queue, upload, download, move, delete, share, and public-link behavior remains intact.
- Tests and JS syntax check pass.
- No secrets or local test files are committed.
