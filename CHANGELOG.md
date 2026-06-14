# Changelog

All notable, user-facing changes to Sempa are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/), and Sempa follows
[Semantic Versioning](https://semver.org/). Each release is also tagged in git
(`vX.Y.Z`) with auto-generated notes on the
[Releases page](https://github.com/moorew/sempa/releases).

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
