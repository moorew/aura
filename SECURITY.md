# Security

## Automated scanning

Continuous checks run in CI via [`.github/workflows/security.yml`](.github/workflows/security.yml)
on every push/PR to `main` and on a weekly schedule:

| Check | Tool | Covers |
|-------|------|--------|
| SAST (code flaws) | **CodeQL** | Go backend + SvelteKit/TS frontend |
| Go dependency CVEs | **govulncheck** | Only CVEs reachable from your code |
| Deps · IaC · secrets | **Trivy** | npm/Go/Cargo CVEs, Dockerfile + compose misconfig, secrets |
| Secret scanning | **gitleaks** | Full git history |

Dependency update PRs are opened weekly by **Dependabot**
([`.github/dependabot.yml`](.github/dependabot.yml)) for Go, npm, Cargo, Docker,
and GitHub Actions.

Results land in the repo's **Security → Code scanning** tab where available, and
always in the workflow logs.

## Enable the free native GitHub features

In **Settings → Code security and analysis**, turn on:

- **Secret scanning** + **Push protection** — blocks commits containing secrets before they land
- **Dependabot alerts**
- **CodeQL / Code scanning** (auto-enabled by the workflow above)

## Handling secrets

- Never commit `.env`, `.env.local`, keystores (`*.keystore` / `*.jks`), or other
  credentials — these are git-ignored.
- Release signing material is injected in CI from repository secrets
  (`KEYSTORE_BASE64`, `KEYSTORE_PASSWORD`, `KEY_ALIAS`, `KEY_PASSWORD`,
  `GOOGLE_SERVICES_JSON`) — see the release workflows.
- If a secret is ever committed, rotate it first, then purge it from history.

## Reporting a vulnerability

Open a private security advisory via **Security → Advisories**, or contact the maintainer directly.
