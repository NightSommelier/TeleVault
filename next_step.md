# Completed: Upload Queue UI and Background Multi-File Workflow

Status: completed on 2026-05-19.

The previous task added a visible dependency-free foreground upload queue in `backend/internal/httpserver/static/index.html`, updated docs in `docs/development/web.md` and `TeleVault-plan.md`, and passed:

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

Do not redo the completed upload-queue foundation unless a regression is found.

---

# Next Step: Upload Queue Controls and Cleanup UX

## Goal

Make the new upload queue easier to operate after files complete or fail.

The queue currently shows per-file states and keeps processing after failures, but completed and failed rows remain passive. Add lightweight controls so users can retry failed uploads, remove finished/failed rows, and clear completed rows without changing backend upload architecture.

## Context

Recent relevant work:

- Foreground upload queue was added to the embedded web UI.
- Queue processing is intentionally conservative: one active file at a time.
- Queue items track `file`, `parentID`, `status`, `progress`, `message`, and `error`.
- Successful uploads refresh the current folder list.
- Failed uploads do not block later queued files.

Important files:

- `backend/internal/httpserver/static/index.html`
- `docs/development/web.md`
- `TeleVault-plan.md`

## Required Behavior

1. Add queue item controls in the web UI:
   - failed item: `Retry` and `Remove`;
   - completed item: `Remove`;
   - queue panel/global action: `Clear completed` when at least one completed item exists.
2. `Retry` should reset a failed item to `queued`, clear its error, reset progress/message, and restart the queue if it is not already running.
3. `Remove` should remove only inactive items (`complete` or `failed`). Do not allow removing `queued`, `hashing`, `staging`, `telegram`, or `completing` items in this step.
4. `Clear completed` should remove all completed items and leave failed/in-flight/queued items untouched.
5. Retried items must keep their original target folder (`parentID`) and original `File` object.
6. A retry failure should return the item to `failed` and still allow later queued items to continue.
7. Keep the UI dependency-free and avoid backend changes unless a real API gap is discovered.

## Suggested Implementation

In `backend/internal/httpserver/static/index.html`:

- Extend the queue panel header or body with a `Clear completed` button.
- Add DOM references for the new controls.
- Update `renderUploadQueue()` to render per-row action buttons based on item status.
- Add helper functions such as:

```js
retryQueueItem(id)
removeQueueItem(id)
clearCompletedQueueItems()
wireUploadQueueActions()
```

- Make `renderUploadQueue()` call the action-wiring helper after updating `innerHTML`.
- Keep row action handlers data-attribute based, consistent with existing file/share action wiring.

## Constraints

- Do not introduce parallel uploads.
- Do not add cancel-in-flight behavior yet.
- Do not persist queue state across page reloads yet.
- Do not store plaintext files on disk beyond the existing upload request flow.
- Do not change Telegram worker concurrency or retry policy in this step.

## Docs

Update:

- `docs/development/web.md`
- `TeleVault-plan.md`

Mention that the foreground queue supports retry/removal controls, while in-flight cancel and backend worker concurrency remain separate future work.

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

- Select multiple files and confirm queue rows still process one at a time.
- Force one upload failure, retry it, and confirm it keeps the original target folder.
- Remove a failed row and a completed row.
- Clear completed rows while failed rows remain visible.

## Acceptance Criteria

- Failed rows show working `Retry` and `Remove` controls.
- Completed rows show working `Remove` controls.
- `Clear completed` removes only completed rows.
- Retried rows keep their original target folder and can complete successfully.
- Active and queued rows cannot be removed in this step.
- Existing upload queue behavior remains intact.
- No backend API changes are made unless justified by a real missing field.
- No secrets or local test files are committed.
