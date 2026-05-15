# TeleDrive 2.0 Threat Model

Status: initial MVP baseline
Date: 2026-05-15

## MVP Decisions

- Deployment target: self-hosted-first.
- Encryption model: server-side streaming encryption.
- File storage: Telegram stores ciphertext only.
- Metadata: filenames are plaintext for MVP; schema keeps room for encrypted names later.
- Sharing: no public links or user-to-user sharing in MVP.
- Upload path: API streams encrypted file parts to Telegram synchronously for MVP.
- Frontend: current React UI may be adapted after the owner-only encrypted backend flow works.

## Assets

- Telegram session strings.
- Telegram session encryption key.
- Application session and refresh tokens.
- Server age identity.
- User file plaintext while in transit through the API process.
- Encrypted file ciphertext stored in Telegram.
- PostgreSQL metadata, key envelopes, audit events, and file part mappings.

## Trust Boundaries

- Browser/client to API over HTTPS.
- API to PostgreSQL.
- API to Valkey using the Redis-compatible protocol.
- API/worker to Telegram.
- Operator-controlled secrets mounted into the backend runtime.

## In Scope Threats

- Theft or leakage of Telegram session strings.
- Client-visible tokens containing sensitive backend credentials.
- Cross-user file, folder, upload, or download access.
- Plaintext file persistence outside the request stream.
- Uploading plaintext to Telegram.
- Weak cookie/CORS/session handling.
- Missing authorization before metadata or stream access.
- Secret leakage through logs or error responses.
- Abandoned uploads and partial state causing data exposure or storage leaks.

## Out of Scope for MVP

- Client-side zero-knowledge encryption.
- Public links.
- User-to-user sharing.
- Legacy migration from TeleDrive v1.
- Advanced previews/search over encrypted metadata.
- Hosted multi-tenant SaaS controls beyond self-hosted-safe defaults.

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
- Store only ciphertext in Telegram.
- Expire and clean abandoned uploads.
- Emit audit events for login, logout, refresh rotation, upload start, upload complete, download, and delete.

## Backup Requirement

The server age identity and PostgreSQL metadata must be backed up together. Losing either can make encrypted files unrecoverable even if Telegram ciphertext still exists.

## Open Decisions Before Upload Implementation

- Maximum MVP file size.
- Per-request upload timeout and retry behavior.
- Exact Telegram part size.
- Whether a local bootstrap admin flow is required for first setup.
