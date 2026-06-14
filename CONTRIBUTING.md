# Contributing to Sempa

Thanks for your interest in improving Sempa! Bug reports, feature requests, and
pull requests are all welcome.

## Reporting bugs & requesting features

- **Bugs / feature requests:** open a [GitHub Issue](https://github.com/moorew/sempa/issues).
  Please search existing issues first, and include steps to reproduce, expected
  vs. actual behaviour, and your platform (web / Windows / Android) and version.
- **Security vulnerabilities:** do **not** open a public issue — follow the
  process in [SECURITY.md](SECURITY.md) (private vulnerability reporting).

## Contribution process

Sempa uses the standard GitHub fork-and-pull-request workflow:

1. Fork the repository and create a topic branch off `main`
   (e.g. `fix/recurrence-horizon`).
2. Make your change, with tests (see below).
3. Open a pull request against `main`. The **Security** workflow (CodeQL,
   govulncheck, Trivy, gitleaks, zizmor) runs automatically and must pass;
   `main` is protected, so all changes land via reviewed PRs.
4. Keep PRs focused — one logical change per PR is easiest to review.

## Requirements for contributions

- **Tests:** as new functionality is added, add tests for it to the automated
  test suite. PRs that change behaviour should include or update tests.
- **Formatting & linting:**
  - Go: code must be `gofmt`-clean and pass `go vet ./...`.
  - Frontend: must pass `npm run check` (`svelte-check` + TypeScript).
- **Commit messages:** use [Conventional Commits](https://www.conventionalcommits.org/)
  (e.g. `fix(recurrence): …`, `feat(api): …`), matching the existing history.
- **Sign-off (DCO):** sign every commit with `git commit -s`, certifying you
  wrote the change or have the right to submit it under the
  [Developer Certificate of Origin](https://developercertificate.org/).

## Running the test suite

```bash
# Backend (Go)
cd backend
go test ./...

# Frontend (TypeScript / Svelte)
cd frontend
npm install
npm test          # vitest unit tests
npm run check     # svelte-check + type checking
```

See the [Development](README.md#development) section of the README for running
the app locally.

## Licensing of contributions

Sempa is licensed under the **GNU AGPL-3.0-or-later** (see [LICENSE](LICENSE)).
By submitting a contribution you agree that it is licensed under the same terms
(inbound = outbound). You additionally grant the project maintainer the right to
distribute your contribution under alternative license terms (e.g. a commercial
license), so the project can be dual-licensed — this keeps long-term
sustainability options open while the code remains free and open-source.
