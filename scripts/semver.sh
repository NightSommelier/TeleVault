#!/usr/bin/env bash
set -euo pipefail

# Resolve SemVer 2.0.0-compatible build metadata from git state.
# Output format:
# - default: plain version string
# - --env: shell-safe KEY=VALUE lines for eval/export
# - --write-go [path]: write Go constants file for buildinfo package
# - --write-go-stable [path]: write stable Go constants for committing

if ! command -v git >/dev/null 2>&1; then
  echo "git is required" >&2
  exit 1
fi

if [[ ! -f VERSION ]]; then
  echo "VERSION file is required at repository root" >&2
  exit 1
fi

base_version="$(tr -d '[:space:]' < VERSION)"
if [[ ! "${base_version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "VERSION must contain SemVer core only, for example: 0.1.0" >&2
  exit 1
fi

base_tag="v${base_version}"
if git rev-parse -q --verify "refs/tags/${base_tag}" >/dev/null 2>&1; then
  commits_ahead="$(git rev-list --count "${base_tag}..HEAD" 2>/dev/null || echo 0)"
else
  commits_ahead="$(git rev-list --count HEAD 2>/dev/null || echo 0)"
fi

short_sha="$(git rev-parse --short=12 HEAD)"
build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
commit_date="$(git show -s --format=%cI HEAD)"

dirty_suffix=""
if ! git diff --quiet --ignore-submodules -- || ! git diff --cached --quiet --ignore-submodules --; then
  dirty_suffix=".dirty"
fi

if [[ "${commits_ahead}" == "0" && -z "${dirty_suffix}" ]] && git describe --tags --exact-match --match "${base_tag}" >/dev/null 2>&1; then
  version_clean="${base_version}"
else
  version_clean="${base_version}-dev.${commits_ahead}+g${short_sha}"
fi

version="${version_clean}${dirty_suffix}"

if [[ "${1:-}" == "--env" ]]; then
  printf 'APP_VERSION=%q\n' "${version}"
  printf 'APP_COMMIT=%q\n' "${short_sha}${dirty_suffix}"
  printf 'APP_BUILD_DATE=%q\n' "${build_date}"
  exit 0
fi

if [[ "${1:-}" == "--write-go" ]]; then
  target_path="${2:-backend/internal/buildinfo/version_generated.go}"
  cat >"${target_path}" <<EOF
package buildinfo

const (
	generatedVersion = "${version}"
	generatedCommit  = "${short_sha}${dirty_suffix}"
	generatedDate    = "${build_date}"
)
EOF
  echo "wrote ${target_path}"
  exit 0
fi

if [[ "${1:-}" == "--write-go-stable" ]]; then
  target_path="${2:-backend/internal/buildinfo/version_generated.go}"
  cat >"${target_path}" <<EOF
package buildinfo

const (
	generatedVersion = "${version_clean}"
	generatedCommit  = "${short_sha}"
	generatedDate    = "${commit_date}"
)
EOF
  echo "wrote ${target_path}"
  exit 0
fi

if [[ "${1:-}" == "--require-clean-release" ]]; then
  if [[ -n "${dirty_suffix}" ]]; then
    echo "release check failed: git worktree is dirty" >&2
    exit 1
  fi
  if [[ "${commits_ahead}" != "0" ]] || ! git describe --tags --exact-match --match "${base_tag}" >/dev/null 2>&1; then
    echo "release check failed: HEAD must be exactly tag ${base_tag}" >&2
    exit 1
  fi
  echo "release check passed for ${base_tag}"
  exit 0
fi

echo "${version}"
