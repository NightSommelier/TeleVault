# Security Policy

TeleVault is a self-hosted encrypted file management utility with Telegram integration.

## Security Contact

Preferred reporting channel: GitHub Security Advisories.

Fallback email: `sommelier@matrix.co.ua`.

## Reporting a Vulnerability

Report security issues privately to the maintainer. Do not open public issues for exploitable vulnerabilities before a fix is available.

Include:

- affected version or commit;
- deployment shape (API/worker/cleanup, reverse proxy);
- reproduction steps;
- impact and expected behavior;
- logs with secrets removed.

## Security Scope

In scope:

- authentication, session, CSRF, and CORS bypasses;
- authorization bypasses for files, folders, shares, and public links;
- public-link expiry/revoke/password bypass;
- secret leakage in responses, logs, or telemetry;
- Telegram session exposure;
- recovery metadata exposure paths;
- upload staging exposure, path traversal, or unsafe file handling.

Out of scope:

- social engineering and phishing;
- local machine compromise outside TeleVault;
- denial-of-service without a product vulnerability.

## Current Security Model

- Telegram stores ciphertext, not plaintext files.
- Server-side encryption is the current baseline.
- The backend may temporarily process decrypted file data in memory during upload/download operations depending on deployment configuration.
- Current mode is not full zero-knowledge.

## Sensitive Secrets and Data

Treat the following as high-sensitivity:

- `.env` and runtime secrets;
- `APP_AGE_IDENTITY`;
- `TELEGRAM_SESSION_KEY`;
- encrypted Telegram sessions;
- recovery metadata and recovery key material;
- upload staging data;
- PostgreSQL backups with file metadata/part placement.

Do not commit these assets. Do not paste them into public issues, chats, or logs.

## Recovery Map Handling

Current recovery export may include private AGE identity material and Telegram file placement metadata.

- Handle exported recovery metadata as a secret.
- Do not store raw recovery bundles in shared backups without additional encryption.
- Do not attach recovery bundles to tickets or bug reports.

## Terminology

Recovery bundle:
Encrypted recovery/export package containing metadata and recovery material.

## Guarantees and Non-Guarantees

TeleVault focuses on privacy, ownership, self-hosting, encryption, organization, and portability.

TeleVault does not claim unlimited storage, full zero-knowledge guarantees, or bypass of provider limits.

## Hardening References

- [Threat model](./docs/threat-model.md)
- [Recovery bundles](./docs/development/recovery.md)
