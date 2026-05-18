# TeleDrive Vault Project Plan

This document defines the plan for building TeleDrive Vault, a security-first encrypted file storage service that uses Telegram as a storage transport while keeping application sessions, metadata, staging, queues, and access control under the service's control.

Current reference project directory: `/Users/sommelier/teledrive`

New project directory: `/Users/sommelier/teledrive-2`.

Working product name: `TeleDrive Vault`.

Repository and local directory names can remain technical while the product name stabilizes. User-facing docs, UI copy, and architecture documents should use `TeleDrive Vault` unless the project is renamed again later.

## 1. Goal

Build a new, security-first TeleDrive Vault implementation:

- Store files in Telegram only as encrypted ciphertext.
- Avoid putting Telegram sessions into JWTs or client-visible tokens.
- Use a centralized authorization model for all file, folder, share, and admin operations.
- Support safe upload/download streaming.
- Support future migration from the existing TeleDrive project.
- Reuse useful ideas and UI from the current project, but do not copy unsafe backend patterns.

## 1.1 Current Audit Status

Plan audit completed on 2026-05-15. Detailed findings are in `docs/plan-audit.md`.

Foundation started on 2026-05-15:

- Created `docs/threat-model.md`.
- Created Go backend skeleton under `backend/`.
- Added config validation with fail-fast required secrets.
- Added `/healthz` and `/readyz`.
- Connected `/readyz` to PostgreSQL ping.
- Added Docker Compose for PostgreSQL and Valkey.
- Added initial MVP SQL migration.
- Added minimal migration runner under `backend/cmd/migrate`.
- Added config validation unit tests.
- Applied initial migration locally.
- Added HTTP request logging with sensitive header redaction.
- Added auth/session foundation with hashed refresh tokens, secure cookie helpers, refresh/logout endpoints, and Telegram auth endpoint placeholders.
- Added dedicated `TELEGRAM_SESSION_KEY` plan/config support for encrypting Telegram sessions at rest.
- Added `auth_challenges` migration, Telegram auth client interface, phone hashing, and encrypted Telegram session storage helpers.
- Applied `000002_auth_challenges` locally.
- Connected `gotd/td` MTProto client.
- Implemented `POST /auth/telegram/send-code`.
- Implemented code-only `POST /auth/telegram/login` with encrypted Telegram session storage and refresh cookie creation.
- Added encrypted temporary MTProto session storage for Telegram auth challenges.
- Smoke-tested Telegram code-only login successfully against a real account.
- Added refresh-cookie auth middleware and `GET /me`.
- Added auth audit events for Telegram code send, login, refresh, and logout.
- Added double-submit CSRF protection for cookie-authenticated refresh/logout requests.
- Added owner-only files/folders metadata API: `GET /files`, `POST /folders`, `GET /files/{id}`.
- Added upload session API: `POST /uploads`.
- Added upload part API: `POST /uploads/{id}/parts/{part_number}` encrypts request bodies with age, uploads ciphertext to Telegram Saved Messages, and records part metadata.
- Added upload completion API: `POST /uploads/{id}/complete` promotes verified upload sessions into `files` and `file_parts`.
- Smoke-tested single-part upload with `test.png`: encrypted part stored in Telegram Saved Messages and file promoted to ready metadata.
- Added authenticated download API: `GET /files/{id}/download` fetches encrypted Telegram parts, decrypts with age, and streams plaintext.
- Smoke-tested `test.png` download: downloaded SHA-256 matched the local original.
- Added rolling SHA-256 upload integrity state so multipart uploads can verify whole-file checksum without retaining plaintext.
- Smoke-tested checksum-backed upload/download with `integrity.txt`.
- Added cleanup command for abandoned upload sessions and failed-part marking for Telegram upload errors.
- Added `cmd/smoke` for authenticated upload/download smoke tests with checksum comparison.
- Added `.env.example`.
- Added configurable upload part sizing with Telegram document cap and safety margin validation.
- Added best-effort Telegram cleanup for encrypted parts that belong to expired uploads.
- Added opt-in Postgres integration tests for auth/session, files owner isolation, and upload completion persistence.
- Added in-memory rate limiting for Telegram auth endpoints by remote IP and phone hash.
- Added admin settings model/API for global upload limits and per-account Telegram limit overrides.
- Added effective per-account upload limit resolution and Telegram limit probe state fields.
- Added a local admin bootstrap CLI for listing users and promoting or demoting trusted Telegram users without manual SQL.
- Added an explicit admin CLI for Telegram upload-limit probing, with dry-run by default and immediate cleanup of successful probe messages.
- Added Valkey-backed distributed auth rate limiting with in-memory test storage and fail-open logging on Valkey errors.
- Added Telegram QR login API endpoints with start/poll flow using gotd QR auth.

Accepted MVP decisions:

- MVP target: self-hosted-first backend/core.
- MVP encryption: server-side streaming age-compatible encryption.
- MVP metadata: keep filenames plaintext, while preserving schema room for encrypted names later.
- MVP sharing: no public links or user-to-user sharing in the first usable version.
- MVP upload path: stage upload data on the server first, then drain it to Telegram through a durable queue with bounded worker concurrency and retry/backoff.
- MVP frontend: reuse/adapt the existing React UI only after the backend encrypted owner-only flow is stable.
- Upload sizing: adapt per-connection/per-account upload parts to current Telegram limits with a safety margin. Baseline Telegram limits are 2 GB for free accounts and 4 GB for Premium accounts.
- Upload backend: local spool or object storage staging is part of the primary path; direct Telegram transfer happens through the drain worker.
- Admin access: a separate admin panel will manage server-side Telegram accounts and operational settings, with a separate bootstrap path so admin access does not hinge on one Telegram account.
- Auth methods: support Telegram phone/code login now and QR login as an additional method where practical.
- Key model: keep the app-controlled age identity for server-side encryption in MVP; per-user age key export/import is a future private-vault mode.

Critical plan corrections from the audit:

- Phase 0 decisions are implementation blockers.
- Key management must be concrete before uploads are enabled.
- Strict CORS, secure cookies, CSRF decision, rate limits, config validation, and secret redaction belong in the foundation phases, not only final hardening.
- Upload sessions need explicit database state.
- Sharing and public-link schema/API should be deferred until after MVP unless a later phase starts.

## 2. Why a New Project

The current backend is useful as a proof of concept, but its security model is not suitable for a private encrypted file storage product.

Main reasons to start a new backend/core:

- Files are uploaded to Telegram without application-level encryption.
- Telegram session data is embedded in JWT payloads.
- Shared file access relies on encrypted Telegram session material.
- Cookies are not configured with strong security flags.
- CORS allows arbitrary origins with credentials.
- Download cache stores plaintext files on disk.
- Access control checks are scattered across request handlers.
- Some file operations lack strict ownership checks.

## 3. What to Reuse From the Current Project

Use the current project at `/Users/sommelier/teledrive` as a reference, not as code to copy blindly.

### Reuse or adapt

- Telegram upload/download flow ideas from `api/src/api/v1/Files.ts`.
- Folder, breadcrumb, file listing, and dashboard concepts.
- Existing React UI as a temporary frontend.
- Existing data model concepts: users, files, config, usage tracking.
- Saved Telegram destination/channel idea.
- Existing Docker/deployment docs as operational reference.

### Do not reuse directly

- JWTs containing Telegram session strings.
- `signed_key` model that stores encrypted Telegram session data for sharing.
- Wildcard CORS with credentials.
- Cookies without `httpOnly`, `secure`, and `sameSite`.
- Plaintext `.cached` file cache.
- Query-to-Prisma dynamic filtering model.
- Client-controlled `uploadBeta` Telegram message metadata.
- Scattered per-handler authorization logic.

## 4. Recommended Stack

### Backend

- Language: Go.
- Router: `chi`.
- Database: PostgreSQL.
- Migrations: `goose`, `atlas`, or `tern`.
- SQL layer: `sqlc` preferred for explicit SQL and compile-time checking.
- Cache/jobs: Valkey using the Redis-compatible protocol, optionally with a compatible background job library.
- Config: environment variables with strict validation at startup.

### Encryption

- File encryption format: age-compatible encryption.
- Go library: `filippo.io/age`.
- CLI compatibility: `rage` can decrypt/encrypt compatible age files for admin/debug/self-host workflows.
- Password KDF: Argon2id or scrypt for password-protected links.

### Frontend

- Reuse the current React frontend initially if possible.
- Add a compatibility layer only where safe.
- Later migrate to a cleaner frontend if needed.

## 5. Target Architecture

```text
web client
   |
   v
Go API ---- PostgreSQL
   |            |
   |            +-- metadata, users, sessions, file parts, encrypted key envelopes
   |
   +---- Valkey
   |       +-- rate limits, jobs, short-lived cache
   |
   +---- Telegram worker/client
           +-- uploads/downloads encrypted file parts to Telegram
```

Main components:

- `api`: HTTP API.
- `staging`: local spool or object storage abstraction for upload payloads before Telegram draining.
- `queue`: durable job leasing, retry scheduling, and upload-part draining.
- `worker`: Telegram upload/download and background migration jobs.
- `crypto`: age encryption/decryption and key wrapping.
- `auth`: app sessions and Telegram login flow.
- `files`: folders, metadata, uploads, downloads.
- `shares`: user shares, public links, password-protected links.
- `policy`: centralized authorization checks.
- `audit`: security-relevant event logging.

## 6. Security Model

### Application sessions

- Use opaque session IDs or short-lived access tokens without Telegram session material.
- Store refresh tokens hashed in the database.
- Rotate refresh tokens.
- Allow session revocation.

### Cookies

Production cookies must use:

- `HttpOnly: true`
- `Secure: true`
- `SameSite: Lax` or `Strict`
- bounded expiration

If cookie-based auth is used, add CSRF protection for state-changing requests.

### Telegram sessions

- Store Telegram session strings server-side only.
- Encrypt Telegram sessions at rest using an application master key or KMS.
- Never place Telegram sessions in JWTs, public links, logs, or frontend responses.

### CORS

Use a strict allowlist:

- production app domain
- production API domain, if needed
- localhost development origins only in development

No wildcard origin with credentials.

### Authorization

Use a central policy layer instead of ad hoc checks in handlers:

- `CanReadFile(user, file)`
- `CanWriteFile(user, file)`
- `CanMoveFile(user, file, targetParent)`
- `CanShareFile(user, file)`
- `CanDeleteFile(user, file)`
- `CanAdmin(user)`

Every endpoint must call policy before accessing or modifying data.

### Queueing and staging

Large uploads should not depend on in-memory buffering or on Telegram accepting every part immediately.

- Stage file data on the server first, either on local disk spool or object storage.
- Persist upload and part state in SQL so uploads survive process restarts.
- Drain staged parts to Telegram from a worker through a durable queue.
- Use bounded concurrency and retry with backoff when Telegram slows down, times out, or returns `FLOOD_WAIT`.
- Treat the queue as part of the product: it keeps uploads resumable, debuggable, and safer for Telegram accounts.
- Prefer PostgreSQL for the primary queue because it supports robust row-claiming patterns such as `SKIP LOCKED`.
- Support MariaDB for portability, with a minimum recommended version of 10.6 for `SKIP LOCKED`-based queue claims on InnoDB.
- Support SQLite3 for single-node, local, embedded, or lightweight deployments where simplicity matters more than multi-worker queue throughput.
- Do not present SQLite3 as the ideal production queue backend for high concurrency; it is the compatibility option, not the scalability target.

## 7. Encryption Model

### MVP model: server-side encryption

For the first version, implement server-side encryption:

1. Client uploads plaintext to the API.
2. API encrypts the stream using age-compatible encryption.
3. API uploads only ciphertext to Telegram.
4. API stores encrypted file metadata and part mapping in PostgreSQL.
5. On download, API fetches ciphertext from Telegram, decrypts it, and streams plaintext to the authenticated client.

This is easier to ship but means the backend can see plaintext during upload/download.

### Future model: client-side encryption

Add client-side encryption later for private vaults:

1. Browser/client encrypts before upload.
2. Backend and Telegram only see ciphertext.
3. Preview/search functionality becomes limited unless metadata is also handled carefully.

### Key handling

Recommended approach:

- Generate a random data key per file.
- Encrypt file content with age-compatible encryption.
- Store file key envelopes in `file_keys`.
- For sharing, wrap the file key for each recipient.
- For public links, use a separate link key or password-derived key.

MVP key handling decision:

- Load a server age identity from `APP_AGE_IDENTITY` or an equivalent mounted/KMS-backed secret.
- Derive the public recipient at startup and refuse to start if the identity is missing or invalid.
- Encrypt uploaded file streams to the server recipient.
- Store encryption scheme, recipient/key envelope metadata, and checksums needed for verification.
- Document backup/restore of the server age identity before enabling uploads.
- Do not store plaintext file keys or Telegram session material in logs, tokens, cookies, links, or frontend responses.

### Metadata

Decide early whether to encrypt metadata:

- MVP can keep file names plaintext for usability.
- Privacy mode should encrypt file names and sensitive metadata.
- Always store checksums carefully; plaintext checksums can reveal file identity.

## 8. Database Draft

Initial tables:

```sql
users
- id
- telegram_id
- username
- display_name
- role
- created_at
- updated_at

sessions
- id
- user_id
- refresh_token_hash
- user_agent
- ip_hash
- expires_at
- revoked_at
- created_at

telegram_sessions
- id
- user_id
- encrypted_session
- storage_peer
- created_at
- updated_at

files
- id
- owner_id
- parent_id
- name_plain
- name_encrypted
- mime_type
- plaintext_size
- ciphertext_size
- type
- status
- encryption_scheme
- checksum
- created_at
- updated_at
- deleted_at

file_parts
- id
- file_id
- part_number
- telegram_peer
- telegram_message_id
- ciphertext_size
- checksum
- created_at

uploads
- id
- owner_id
- parent_id
- name_plain
- mime_type
- plaintext_size
- part_size
- status
- idempotency_key
- error_code
- created_at
- updated_at
- expires_at

upload_parts
- id
- upload_id
- part_number
- plaintext_size
- ciphertext_size
- checksum
- status
- storage_backend
- storage_key
- available_at
- leased_until
- attempts
- last_error
- worker_id
- created_at
- updated_at

file_keys
- id
- file_id
- recipient_type
- recipient_id
- encrypted_key
- algorithm
- created_at

shares
- id
- file_id
- owner_id
- recipient_user_id
- permissions
- expires_at
- revoked_at
- created_at

public_links
- id
- file_id
- token_hash
- password_hash
- permissions
- expires_at
- max_downloads
- download_count
- revoked_at
- created_at

audit_events
- id
- actor_user_id
- action
- resource_type
- resource_id
- ip_hash
- user_agent
- created_at
```

MVP migration note:

- Include `users`, `sessions`, `telegram_sessions`, `files`, `file_parts`, `uploads`, `upload_parts`, `file_keys`, and `audit_events` in the initial backend design.
- Defer `shares` and `public_links` migrations until the sharing phase unless they are actively implemented.
- Add unique constraints for `users.telegram_id`, `(file_id, part_number)`, and `(upload_id, part_number)`.
- Add foreign keys for owners, parents, sessions, file parts, upload parts, and key envelopes.
- Add partial indexes for active sessions and non-deleted files.
- Add queue leasing fields for staged upload parts or jobs: `available_at`, `leased_until`, `attempts`, `last_error`, and `worker_id`.

## 9. API Draft

### Auth

- `POST /auth/telegram/send-code`
- `POST /auth/telegram/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /me`

### Files and folders

- `GET /files?parent_id=...`
- `POST /folders`
- `POST /uploads`
- `PUT /uploads/{upload_id}/parts/{part_number}`
- `POST /uploads/{upload_id}/complete`
- `GET /files/{id}`
- `GET /files/{id}/download`
- `PATCH /files/{id}`
- `DELETE /files/{id}`

### Sharing

- `POST /files/{id}/shares`
- `DELETE /files/{id}/shares/{share_id}`
- `POST /files/{id}/public-links`
- `GET /public/{token}`
- `GET /public/{token}/download`

### Admin

- `GET /admin/config`
- `PATCH /admin/config`
- `GET /admin/users`

## 10. Implementation Roadmap

### Phase 0: Product and threat model

- Accept MVP as self-hosted-first unless a concrete hosted SaaS requirement appears.
- Accept server-side streaming encryption for MVP.
- Accept plaintext filenames for MVP, with schema support for encrypted names later.
- Exclude public links and user-to-user sharing from MVP.
- Define max supported MVP file size and request timeout behavior.
- Define backup/restore requirements for PostgreSQL metadata and the server age identity.
- Write `docs/threat-model.md` before auth and upload implementation starts.

### Phase 1: New project skeleton

- Create a new directory, for example `/Users/sommelier/teledrive-2`.
- Add Go module.
- Add Docker Compose with Postgres and Valkey.
- Add config loader and startup validation.
- Add healthcheck endpoint.
- Add readiness endpoint.
- Add migration tooling.
- Add strict CORS configuration with development-only localhost allowances.
- Add structured logging with secret redaction.
- Add production-safe cookie configuration defaults.

### Phase 2: Auth foundation

- Implement Telegram login flow.
- Store encrypted Telegram sessions server-side.
- Implement secure app sessions.
- Implement refresh token rotation.
- Implement logout and revoke.
- Add auth rate limiting.
- Add CSRF protection if cookie-based auth is used for state-changing requests.
- Add auth tests.

### Phase 3: Files metadata and policy

- Add users, sessions, Telegram sessions, folders/files, upload sessions, file parts, file keys, and audit event tables.
- Implement folder CRUD.
- Implement file listing.
- Implement centralized policy checks.
- Add negative unit tests for cross-user reads, writes, moves, deletes, shares, and downloads.

### Phase 4: Encryption service

- Implement streaming age encryption.
- Implement streaming age decryption.
- Add per-file key generation.
- Store encrypted key envelopes.
- Add checksum verification tests.

### Phase 5: Telegram storage

- Upload encrypted file parts to Telegram through the API streaming path for MVP.
- Store Telegram message IDs in `file_parts`.
- Download encrypted parts from Telegram.
- Stage upload payloads before Telegram transfer.
- Retry transient Telegram failures with leased queue jobs and backoff.
- Add background worker structure for cleanup, queue draining, retries, and future migration jobs.

### Phase 6: Upload/download MVP

- Implement upload session creation.
- Implement part upload and staging.
- Implement upload completion.
- Implement authenticated download with decrypt streaming.
- Implement abandoned upload expiration and cleanup.
- Implement soft delete with best-effort Telegram message deletion.
- Defer complex range support until the basic flow is stable.

### Phase 7: Sharing

Post-MVP only.

- Implement user-to-user shares.
- Implement public links.
- Implement password-protected links.
- Implement expiry and revoke.
- Add audit events.

### Phase 8: Frontend integration

- Point current React frontend to the new API.
- Replace auth flow.
- Replace upload/download flow.
- Display encryption status.
- Keep UI changes minimal initially.

### Phase 9: Legacy migration

- Import users and file metadata from the old project.
- Support legacy unencrypted files in read-only mode, if needed.
- Build a migration job:
  - download old plaintext file from Telegram,
  - encrypt it,
  - upload ciphertext to Telegram,
  - update metadata,
  - verify checksum,
  - optionally delete old Telegram message.

### Phase 10: Hardening

- Expand rate limiting beyond auth.
- Review CSRF coverage.
- Review strict CORS coverage.
- Add security headers.
- Add audit logs.
- Add dependency scanning.
- Add integration tests.
- Add backup/restore documentation.

## 11. MVP Scope

The first usable version should include only:

- Telegram login.
- Secure app sessions.
- Folder listing and creation.
- Upload one file.
- Server-side age encryption.
- Store ciphertext in Telegram.
- Download and decrypt for the owner.
- Basic delete.
- No public sharing yet.
- No legacy migration yet.
- No advanced previews yet.

This validates the most important part: encrypted storage on Telegram.

## 12. Initial Directory Layout for the New Project

Suggested structure for `/Users/sommelier/teledrive-2`:

```text
teledrive-2/
  backend/
    cmd/
      api/
      worker/
    internal/
      auth/
      config/
      crypto/
      db/
      files/
      policy/
      shares/
      telegram/
      audit/
    migrations/
    sql/
    go.mod
  web/
  docker-compose.yml
  README.md
  docs/
    architecture.md
    threat-model.md
    migration-from-teledrive-v1.md
```

## 13. Immediate Next Steps

1. Lock in the adaptive Telegram limit policy and durable queue/staging model:
   - probe-driven per-account caps,
   - rolling safety margins,
   - bounded concurrent uploads,
   - backoff/retry rules for `FLOOD_WAIT` and temporary Telegram slowdown,
   - local spool or object storage staging before Telegram drain,
   - portable SQL queue semantics across PostgreSQL, MariaDB, and SQLite3.
2. Build the first usable web UI for auth and owner-only file browsing/upload smoke flows.

## 13.1 Deferred Planned Auth Work

- Add Telegram 2FA password handling for accounts that require it. This is planned, but it is not blocking the current MVP flow because the current test account does not have Telegram 2FA enabled yet.

## 13.2 Deferred Portability Work

- Add optional database backends beyond PostgreSQL, such as SQLite and MariaDB. This is intentionally deferred until the MVP schema and query patterns stabilize.

## 14. Reference Files in Current Project

Useful current files to inspect while building the new implementation:

- `/Users/sommelier/teledrive/api/src/api/v1/Files.ts`
- `/Users/sommelier/teledrive/api/src/api/v1/Auth.ts`
- `/Users/sommelier/teledrive/api/prisma/schema.prisma`
- `/Users/sommelier/teledrive/web/src/pages/dashboard/index.tsx`
- `/Users/sommelier/teledrive/web/src/pages/dashboard/components/Upload.tsx`
- `/Users/sommelier/teledrive/web/src/utils/Download.ts`

These files should be treated as references for behavior and UI expectations, not as security patterns to preserve.
