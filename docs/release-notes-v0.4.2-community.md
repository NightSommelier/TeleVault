# TeleVault v0.4.2 Community Release Notes

Released: 2026-05-29

Tag: `v0.4.2`

## Highlights

- remembered-device re-entry with a `Continue as ...` flow and an explicit
  forget-device action;
- local challenge methods for repeated login, including passkeys, TOTP,
  recovery codes, and an optional local password fallback;
- Telegram-disconnected safety mode that keeps metadata available while
  blocking mutations and downloads until the Telegram session is restored;
- local 2FA management with TOTP enrollment, recovery-code lifecycle, and
  passkey naming and removal.

## Community Behavior

- Community remains self-hosted and centered on one owner Telegram identity
  per instance;
- local license verification remains the primary access decision;
- data access, export, and recovery remain available when the instance falls
  back to Community behavior.

## Upgrade Notes

- back up the database and application secrets before upgrading;
- apply the database changes through the normal deployment migration steps;
- verify login, remembered-device re-entry, read-only safety mode, and recovery
  export after the upgrade.

## Validation

- backend tests and static checks passed;
- embedded web checks and script syntax validation passed;
- release working-tree checks passed.
