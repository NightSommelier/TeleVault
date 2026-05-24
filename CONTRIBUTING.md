# Contributing

TeleVault welcomes focused contributions that improve the public repository, deployment experience, and security posture.

## Code Style

- Keep changes small and scoped.
- Follow the existing Go, HTML, and Markdown style already used in the repository.
- Run `gofmt` on Go code when needed.
- Avoid adding new dependencies unless they remove a real maintenance burden.

## Issues

- Include the current version or commit.
- Include the deployment shape, such as local compose, server compose, or a proxied instance.
- Include reproduction steps and expected behavior.
- Remove secrets from logs, screenshots, and pasted config before sharing.

## Pull Requests

- Prefer one behavior or documentation concern per pull request.
- Update docs when behavior, security guidance, or product positioning changes.
- Include tests or verification steps when code changes are involved.
- Run `git diff --check` before opening a PR.

## Security

- Do not file public issues for exploitable vulnerabilities.
- Use the security contact in [SECURITY.md](./SECURITY.md) for responsible disclosure.
- Never attach recovery bundles, Telegram sessions, AGE identities, or `.env` files to issues or PRs.
