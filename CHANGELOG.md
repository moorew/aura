# Changelog

All notable, user-facing changes to Sempa are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/), and Sempa follows
[Semantic Versioning](https://semver.org/). Each release is also tagged in git
(`vX.Y.Z`) with auto-generated notes on the
[Releases page](https://github.com/moorew/sempa/releases).

## [1.0.114] - 2026-06-14

### Fixed
- **Installer no longer aborts when you enter a custom value.** `install.sh` ran
  under `set -e`, and its `ask_default` helper returned a non-zero status whenever
  you typed anything other than the default (App URL, host port, or username),
  silently exiting the script right after the first prompt. Accepting every
  default happened to work, which hid the bug.
- Made `install.sh` portable to hosts with BusyBox `grep` (e.g. minimal/Alpine
  systems): replaced `grep -oP` for the Docker version and URL port with
  bash-native parsing.
- Hardened the "update existing install" backup step so it can't abort when only
  one of `.env` / `.env.local` is present.

### Changed
- **Clarified Tailscale setup docs.** The README and installer now explain that
  the bundled `ts-sempa` sidecar joins the tailnet as its own dedicated node
  (`sempa.<your-tailnet>.ts.net`) rather than reusing the host machine's name,
  and document the MagicDNS/HTTPS and `tag:container` prerequisites. Removed the
  incorrect manual `tailscale cert` step (Serve provisions the cert automatically).
- Relicensed under AGPL-3.0; added CONTRIBUTING and this changelog.
- Routine dependency updates.

### Added
- Auto-tagging workflow: bumping the version in `frontend/package.json` on `main`
  now cuts the matching `vX.Y.Z` tag and kicks off the release builds.

## [1.0.113] - 2026-06-14

### Fixed
- **Recurring tasks now appear on future days across all platforms.** Recurring
  instances were generated lazily server-side only when a web client requested a
  given week, so offline-first desktop/Android clients (which read the local DB)
  saw a daily task "end" after the current week. A background poller now
  proactively materialises the current week plus the next two weeks.

### Security
- Updated Go toolchain to 1.25.11, fixing reachable standard-library
  vulnerabilities (GO-2026-5037/5038/5039, GO-2026-4986, GO-2026-4971).
- Updated `go-chi/chi` to v5.2.2, fixing GO-2025-3770 (open redirect).
- Added continuous security scanning (CodeQL, govulncheck, Trivy, gitleaks,
  zizmor, OpenSSF Scorecard) and Dependabot; pinned all GitHub Actions to commit
  SHAs.
