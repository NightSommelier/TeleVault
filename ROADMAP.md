# TeleVault Roadmap

TeleVault is a self-hosted encrypted file management utility with Telegram integration. This roadmap summarizes the public product direction and stays aligned with the frozen positioning rules in the documentation.

## Current Baseline

What already works:

- self-hosted Docker Compose deployment;
- Telegram login with phone, QR, and 2FA;
- encrypted upload and download flow;
- folder and file management;
- staged uploads drained by a background worker;
- public links, password links, expiry, revoke, and download limits;
- internal sharing;
- admin upload settings;
- recovery export/import foundation;
- persistent application logs;
- no telemetry by default.

## Near Term

The next work should focus on stability and first-release quality:

- harden large upload reliability and progress reporting;
- improve resume and retry behavior for staged uploads;
- add local TeleVault MFA for Community and higher editions: TOTP and/or WebAuthn with 10 recovery codes;
- add offline-first commercial licensing foundation: local signed license verification and backend feature gates;
- complete recovery bundle UX and documentation;
- expand smoke coverage for auth, upload, download, public links, and recovery;
- document backup and restore for database state, AGE identity, Telegram sessions, logs, and recovery bundles;
- keep the public repository onboarding simple and visually clear.

## Community

Community should remain the complete personal edition:

- one workspace;
- one connected Telegram account;
- one bound Community owner identity that can sign in from multiple browsers/devices;
- CLI included;
- basic Desktop included;
- stable and practical for real deployments;
- self-hosted-first and privacy-focused.

## Pro

Pro should add convenience for power users and homelabs:

- advanced workflows;
- advanced synchronization;
- multi-account management;
- automation features;
- maintenance features;
- advanced integrations;
- richer recovery and administration tooling;
- advanced Desktop workflows.

## Licensing Infrastructure (Planned)

Commercial licensing should be built around a separate licensing server and local instance verification:

- separate licensing server repository and deployment;
- signed license artifact bound to `instance_id` and verified locally in TeleVault;
- no mandatory runtime dependency on licensing-server reachability;
- optional online renewals later, but no mandatory heartbeat checks;
- graceful expiration fallback to Community behavior without data lockout;
- backup and restore guidance must include `instance_id` and installed license artifacts to avoid accidental loss after server reinstall/migration.

## Team

Team is roadmap-only for now:

- shared workspaces;
- collaboration tooling;
- policy controls;
- administrative workflows.

Team stays lightly positioned until the base product and Pro tier are stable.

## Long Term

Longer-term work should stay optional and architecture-driven:

- optional private vault mode with client-side encryption, but no zero-knowledge claim unless it is technically true for the deployed mode;
- optional FUSE mount as a separate process;
- richer recovery metadata and migration flows;
- possible database portability exploration after PostgreSQL remains stable;
- future shared or split-storage workflows only if the trust model stays explicit.

## Product Guardrails

TeleVault should not drift toward these directions:

- hosted cloud service;
- unlimited storage marketing;
- Telegram cloud replacement;
- Telegram-based vault workflow;
- storage resale;
- Telegram infrastructure monetization;
- hidden telemetry;
- hardware-locked DRM or online-only activation.

If a future feature or doc makes TeleVault look more like a hosted storage provider, it should be rewritten toward self-hosting, privacy, encryption, and user-controlled workflows.
