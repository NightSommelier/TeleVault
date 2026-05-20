# Next Step: User Upload Preferences

## Agent Workflow Contract

This file is the handoff contract for the implementation agent.

- Do not make commits.
- Do not push.
- Do not rewrite history.
- Leave all implementation changes as a normal workspace diff.
- After finishing, report:
  - what files changed;
  - what behavior changed;
  - what tests/checks were run and their result;
  - any blocker or skipped acceptance criterion.
- The reviewer/maintainer will inspect the diff, update this file for the next task, and make the commit.
- If the task is ambiguous, stop and write the specific question instead of implementing a broad guess.

## Goal

Add user-level upload preferences for size and pacing parameters while keeping administrator settings authoritative as safe defaults and hard bounds.

The current system has:

- global admin upload settings in `admin_settings`;
- admin-managed per-account Telegram limits in `telegram_account_limits`;
- detected Telegram document caps from probing;
- an effective upload policy resolver used when an upload session is created.

The missing piece is a normal user-facing preference layer. Users should be able to tune their own preferred upload behavior without being able to exceed admin/account safety limits.

## Effective Policy Order

Resolve upload settings in this order:

1. built-in safe defaults;
2. global admin defaults from `admin_settings`;
3. admin per-account limits and detected Telegram caps from `telegram_account_limits`;
4. user upload preferences, clamped to the effective admin/account bounds.

User preferences must never:

- raise `telegram_document_limit_bytes`;
- bypass the effective safety margin;
- create an application part larger than the effective document limit minus the effective safety margin;
- set negative speed/cooldown values;
- set invalid concurrency values.

## Required Behavior

1. Add persistent user upload preferences.
2. Support nullable preference fields so a user can inherit the admin default for any individual field.
3. Add authenticated owner endpoints for the current user's upload preferences:
   - read current preferences plus the effective policy;
   - update preferences;
   - reset preferences back to inherited defaults.
4. Include at least these fields:
   - preferred max application part size bytes;
   - preferred target upload bytes per second;
   - preferred cooldown between parts in milliseconds;
   - preferred max parallel uploads, clamped by admin policy if a hard cap exists.
5. Keep admin/account document limit and safety margin authoritative.
6. Use the user preferences in upload session creation and progress policy responses.
7. Update docs to explain admin defaults vs user preferences.
8. Do not change Telegram upload worker queue semantics beyond consuming the already resolved effective policy.

## Suggested Database Shape

Add a migration with a separate table, for example:

```sql
CREATE TABLE user_upload_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    preferred_upload_part_size_bytes BIGINT,
    preferred_max_parallel_uploads INTEGER,
    preferred_target_upload_bytes_per_second BIGINT,
    preferred_cooldown_between_parts_ms INTEGER,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_upload_preferences_positive_part_size CHECK (
        preferred_upload_part_size_bytes IS NULL OR preferred_upload_part_size_bytes > 0
    ),
    CONSTRAINT user_upload_preferences_positive_parallel CHECK (
        preferred_max_parallel_uploads IS NULL OR preferred_max_parallel_uploads > 0
    ),
    CONSTRAINT user_upload_preferences_nonnegative_target_rate CHECK (
        preferred_target_upload_bytes_per_second IS NULL OR preferred_target_upload_bytes_per_second >= 0
    ),
    CONSTRAINT user_upload_preferences_nonnegative_cooldown CHECK (
        preferred_cooldown_between_parts_ms IS NULL OR preferred_cooldown_between_parts_ms >= 0
    )
);
```

Use the repo's migration style and numbering.

## Files To Inspect

- `backend/migrations/`
- `backend/internal/uploads/store.go`
- `backend/internal/uploads/http.go`
- `backend/internal/adminsettings/store.go`
- `backend/internal/adminsettings/http.go`
- `backend/internal/httpserver/router.go`
- `backend/internal/httpserver/static/index.html`
- `docs/development/admin.md`
- `docs/development/uploads.md`
- `TeleVault-plan.md`

## Implementation Notes

- Prefer a small backend-first implementation. A minimal UI can be a simple settings section or modal if it fits cleanly; otherwise document the endpoint and leave richer UI as a follow-up.
- Keep the API response explicit: return stored user preferences separately from the resolved effective policy so the UI can show inherited values clearly.
- Clamp preferences at resolution time, not only at write time, because admin settings may change later.
- Reject obviously invalid JSON values at write time with clear errors.
- Avoid exposing Telegram peer/session secrets in policy responses.

## Tests To Add Or Update

Add focused coverage for:

- migration-backed persistence if there is an existing integration test pattern;
- effective policy resolution with no user preferences;
- effective policy resolution with user preferences inside admin bounds;
- clamping when user preferences exceed effective admin/account bounds;
- invalid preference updates.

If UI changes are included, also validate embedded JavaScript syntax.

## Verification

Run:

```sh
cd backend
go test ./... -count=1
```

If the embedded web UI changes, validate JavaScript syntax, for example:

```sh
awk '/<script>/{flag=1;next} /<\/script>/{flag=0} flag' backend/internal/httpserver/static/index.html > /tmp/televault-index.js
node --check /tmp/televault-index.js
```

Finally run:

```sh
git diff --check
```

## Acceptance Criteria

- Users can store nullable upload preferences.
- Upload creation uses resolved admin/account bounds plus user preferences.
- Effective policy responses show the resolved result.
- User preferences cannot bypass admin/account safety limits.
- Existing admin settings and per-account limit behavior remains compatible.
- Relevant docs explain the precedence model.
- `go test ./... -count=1` passes.
- `git diff --check` passes.
