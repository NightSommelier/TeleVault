# TeleVault Threat Model

Status: current implemented baseline
Date: 2026-05-19

## MVP Decisions

- Deployment target: self-hosted-first.
- Encryption model: server-side streaming encryption.
- File storage: Telegram stores ciphertext only.
- Metadata: filenames are plaintext for MVP; schema keeps room for encrypted names later.
- Sharing: user-to-user shares, public links, expiring links, revoke, and password-protected public links are implemented and must be treated as in-scope attack surface.
- Upload path: API encrypts and stages file parts locally, then a leased worker queue drains ciphertext to Telegram.
- Frontend: embedded web UI is available and must follow the same cookie, CSRF, authorization, and sharing controls as direct API clients.

## Assets

- Telegram session strings.
- Telegram session encryption key.
- Application session and refresh tokens.
- Server age identity.
- User file plaintext while in transit through the API process.
- Local staged ciphertext waiting for Telegram drain.
- Encrypted file ciphertext stored in Telegram.
- PostgreSQL metadata, key envelopes, audit events, and file part mappings.
- Internal file shares and public-link token hashes.
- Public-link password hashes and salts.
- Per-user recovery bundles and AGE key material once recovery export is implemented.

## Trust Boundaries

- Browser/client to API over HTTPS.
- API to PostgreSQL.
- API to Valkey using the Redis-compatible protocol.
- API to local upload staging.
- Worker to local upload staging.
- Worker to Telegram.
- Operator-controlled secrets mounted into the backend runtime.

## In Scope Threats

- Theft or leakage of Telegram session strings.
- Client-visible tokens containing sensitive backend credentials.
- Cross-user file, folder, upload, or download access.
- Plaintext file persistence outside the request stream.
- Unbounded staged ciphertext growth if Telegram slows down or rejects uploads.
- Uploading plaintext to Telegram.
- Weak cookie/CORS/session handling.
- Missing authorization before metadata or stream access.
- Share bypass, revoked-share access, expired-share access, and public-link token brute force.
- Password-protected public link bypass or weak password verification.
- Public-link leakage through logs, referrers, browser history, screenshots, or user forwarding.
- Secret leakage through logs or error responses.
- Abandoned uploads and partial state causing data exposure or storage leaks.
- Recovery bundle leakage, because bundles may contain enough metadata and key material to restore a user's vault.

## Out of Scope for MVP

- Client-side zero-knowledge encryption.
- Advanced previews/search over encrypted metadata.
- Hosted multi-tenant SaaS controls beyond self-hosted-safe defaults.
- Shared folders, shared upload spaces, shared-channel mode, and split-storage mode.
- FUSE mount mode.

## Product Claims Guardrails

- Do not claim unlimited storage.
- Do not claim bypass of provider limits.
- Do not claim full zero-knowledge in the current server-side encryption mode.

## Required Controls

- Never put Telegram session material in JWTs, cookies, public links, logs, or frontend responses.
- Encrypt Telegram sessions at rest before storing them.
- Use a separate `TELEGRAM_SESSION_KEY` for Telegram session encryption at rest.
- Refuse startup without required session secrets and server age identity.
- Use secure cookie defaults outside local development.
- Use double-submit CSRF protection for cookie-authenticated state-changing requests.
- Use strict CORS allowlists when credentials are enabled.
- Rotate refresh tokens and store only hashes.
- Authorize before returning file metadata, key metadata, Telegram part references, or streams.
- Authorize internal share download using the file owner session, not the current reader session.
- Store only public-link token hashes, never raw public-link tokens.
- Require expiry/revoke checks before public-link metadata or download access.
- Use memory-hard password hashing for password-protected public links.
- Keep public-link passwords out of logs, audit payloads, and persisted plaintext storage.
- Store only ciphertext in Telegram.
- When Telegram-side artifact metadata is intentionally masked, use deterministic decoy names, MIME types, and reversible wrappers so Telegram does not see an obvious `age-encryption.org/v1` header at byte zero. This hides the raw age fingerprint and original names, but it does not make the object a fully valid media file, and it does not protect against size analysis, traffic analysis, or a motivated attacker who knows TeleVault's wrapper format.
- Store only ciphertext in upload staging.
- Protect `UPLOAD_STAGING_DIR` with owner-only filesystem permissions and exclude it from Git/backups unless intentionally backed up with the database and age identity.
- Use durable queue leases, bounded worker concurrency, and retry/backoff for transient Telegram failures.
- Respect Telegram `FLOOD_WAIT` delays before retrying staged parts.
- Expire and clean abandoned uploads.
- Emit audit events for login, logout, refresh rotation, upload start, upload complete, download, delete, share create/revoke, public-link create/revoke, and admin setting changes.
- Export recovery bundles only after explicit authenticated user action and protect them as high-sensitivity secret material.

## Backup Requirement

The server age identity and PostgreSQL metadata must be backed up together. Losing either can make encrypted files unrecoverable even if Telegram ciphertext still exists.

Per-user recovery bundles are planned as an additional restore path. They must be versioned and importable without silently overwriting older snapshots. If a bundle contains a user's AGE private key material or an export envelope for it, it must be handled as a secret with the same care as the server age identity.

## Open Decisions Before Upload Hardening

- Maximum MVP file size.
- Adaptive per-account worker concurrency.
- Production staging backend beyond local spool, such as S3 or Garage.
- Whether a local bootstrap admin flow is required for first setup.
- Recovery bundle JSON schema, export authorization policy, and restore conflict behavior.
- Per-user AGE key lifecycle for existing users and future users.
