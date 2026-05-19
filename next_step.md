# Next Step: Adaptive Upload Worker Concurrency and Pacing

## Goal

Finish upload worker hardening so the instance can keep draining parts safely when Telegram slows down or when multiple uploads are active at the same time.

Right now the worker drains parts sequentially, but the plan already calls for bounded concurrency, per-account pacing, and adaptive backoff when Telegram starts throttling. Implement that next, without changing the storage model unless a test proves it is required.

## Context

Relevant current behavior:

- the API already stages encrypted parts locally and the worker drains them to Telegram;
- queue state, retry metadata, and active worker ids are already exposed;
- admin settings already carry upload policy fields such as target upload rate, cooldown, and max parallel uploads;
- `FLOOD_WAIT_X` handling already exists, but the worker still needs better account-aware pacing and concurrency control.

Important files:

- `backend/internal/uploads/worker.go`
- `backend/internal/uploads/http.go`
- `backend/internal/uploads/store.go`
- `backend/internal/adminsettings/`
- `backend/internal/integration/`
- `docs/development/uploads.md`
- `TeleVault-plan.md`

## Required Behavior

1. Add bounded worker concurrency for draining staged parts, driven by the existing policy fields.
2. Keep per-upload part ordering intact.
3. Make the worker respect current account pacing when it sees backlog or Telegram slowdowns.
4. Continue honoring `FLOOD_WAIT_X` by pushing the next retry forward instead of retrying aggressively.
5. Keep queue progress visible enough for the frontend to explain why work is queued, leased, or deferred.
6. Do not introduce a broad schema migration unless a concrete bug or missing field forces it.
7. Do not regress the current staged-upload flow, retry controls, or privacy-masking wrapper work.

## Suggested Implementation

Use the existing queue and settings first.

Good shape:

- cap active drain work with `max_parallel_uploads`;
- keep one lease per part and one worker slot per concurrent upload;
- let the worker back off when the effective upload duration or queue pressure exceeds the configured pacing;
- preserve the current retry schedule for transient Telegram errors;
- keep single-upload part ordering deterministic.

## Constraints

- No change that breaks existing uploads or recovery paths.
- No hidden behavior that surprises the user with unbounded throttling.
- No broad rewrite of the queue unless the current structure cannot support the policy cleanly.
- Keep the implementation small enough to test with the current integration and unit test style.

## Docs

Update:

- `docs/development/uploads.md`
- `TeleVault-plan.md`

Document how the worker decides when to drain, when to wait, and how the configured limits affect throughput.

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

If the embedded web UI changes, also validate the JS syntax.

## Acceptance Criteria

- The worker obeys `max_parallel_uploads` or the equivalent configured concurrency cap.
- Upload ordering remains stable inside a single upload session.
- Telegram slowdowns push work back through retry/backoff instead of causing tight retry loops.
- Queue progress still explains queued, leased, failed, and ready states correctly.
- Tests cover the new pacing/concurrency behavior.
- `go test ./... -count=1` passes.
- `git diff --check` passes.
