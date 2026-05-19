# Next Step: Upload Policy Visibility in Progress API

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

Expose the effective upload policy in the upload status/progress API so the frontend can explain why parts are queued, leased, or deferred.

The worker already drains with bounded concurrency and strict per-upload ordering. The next step is to surface the policy that controls that behavior instead of leaving the UI to infer it.

## Current State

- `backend/internal/uploads/http.go` already returns queue progress, active workers, retry timing, and completion readiness.
- `backend/internal/uploads/store.go` already resolves the effective upload policy fields for claimed work:
  - `max_parallel_uploads`;
  - `target_upload_bytes_per_second`;
  - `cooldown_between_parts_ms`.
- `backend/internal/adminsettings/` already stores the global and per-account policy values.
- The frontend currently knows whether parts are queued or leased, but not why the instance is pacing uploads the way it is.

## Required Behavior

1. Add the effective upload policy to the upload progress/status response.
2. Keep the values aligned with the same policy resolution used by the worker.
3. Make the response explicit enough for the frontend to explain:
   - the current concurrency cap;
   - the current target upload rate;
   - the current cooldown between parts.
4. Do not expose secrets, session material, Telegram peer ids, or internal database ids that the UI does not already need.
5. Do not change the concurrency logic from the previous task.

## Suggested Implementation

Prefer a small API extension:

- extend the upload handler response with a dedicated policy object;
- source the values from the same settings or policy resolution path already used by upload draining;
- add tests for the response shape and the resolved values;
- keep the frontend changes minimal and utilitarian.

If the upload handler cannot access the effective policy cleanly, add a focused provider to the handler settings rather than duplicating store logic in HTTP code.

## Files To Inspect

- `backend/internal/uploads/http.go`
- `backend/internal/uploads/http_test.go`
- `backend/internal/uploads/store.go`
- `backend/internal/adminsettings/`
- `backend/internal/httpserver/static/index.html`
- `docs/development/uploads.md`
- `TeleVault-plan.md`

## Tests To Add Or Update

Add focused tests for:

- the progress response includes the policy fields;
- the values match the effective settings for the current user/account;
- existing progress fields are unchanged;
- no secret or low-level transport data is exposed.

If the embedded web UI is updated, also validate the embedded JavaScript syntax.

## Docs

Update:

- `docs/development/uploads.md`
- `TeleVault-plan.md`

Document that the upload status page now shows the effective policy driving queueing and pacing.

## Verification

Run:

```sh
cd backend
go test ./... -count=1
```

Then run:

```sh
git diff --check
```

If the embedded web UI changes, also validate the JavaScript syntax.

## Acceptance Criteria

- Upload progress/status responses include the effective upload policy.
- The concurrency cap and pacing values match the effective policy.
- Existing queue state fields still work.
- No new sensitive data is exposed in the response.
- Tests cover the new response fields and value resolution.
- `go test ./... -count=1` passes.
- `git diff --check` passes.
