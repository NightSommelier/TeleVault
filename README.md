# TeleVault

Self-hosted encrypted file management utility with Telegram integration.

Built for self-hosters, homelabs, Linux users, privacy-focused users, power users, and archival/synchronization enthusiasts.

## Product Identity

- Primary identity: self-hosted encrypted file management utility.
- Secondary identity: encrypted file management vault.
- Telegram integration is backend integration and synchronization transport, not hosted storage provided by TeleVault.

## Edition Summary

- Community (AGPL-3.0): complete personal edition, one workspace, one connected Telegram account.
- Pro (planned): advanced workflows, advanced synchronization, multi-account management, automation features, maintenance features, advanced integrations.
- Team (roadmap): future collaboration tooling, shared workspaces, policy controls, and administrative workflows.

Community remains fully usable and practical for real deployments. It is not a demo edition and should not be artificially degraded.

## Security and Privacy

- Telegram stores encrypted artifacts (ciphertext), not original plaintext files.
- Processing may require temporary processing-time access depending on deployment and configuration.
- Current architecture is not full zero-knowledge mode.
- No telemetry by default.

See:

- [SECURITY.md](/home/sommelier/televault/SECURITY.md)
- [Threat model](/home/sommelier/televault/docs/threat-model.md)

## Compliance and Disclaimer

TeleVault is not affiliated with Telegram.

Users are responsible for complying with Telegram Terms of Service.

TeleVault does not provide hosting/storage infrastructure; users connect their own Telegram accounts.
