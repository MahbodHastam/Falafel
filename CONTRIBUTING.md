# Contributing

Thanks for helping. Falafel is deliberately small; changes that keep it that way are
the easiest to merge.

## Setup

Requirements: Go 1.26+, Node.js 20.19+ (22.12+ or 24+), pnpm 10+.

```bash
make install   # web dependencies
make dev       # Go API on :8080, Vite on :5173
```

## Before you open a pull request

```bash
make check     # gofmt, go vet, web typecheck, Go and Vitest suites
make e2e       # needs `pnpm exec playwright install chromium` once
```

Run `make e2e` whenever you touch transfer, signaling, or room lifecycle code.
CI runs the same checks plus `govulncheck` and `pnpm audit`.

## Guidelines

- **Add a test with the change.** Go tests sit next to the code. Browser-side logic is
  tested with Vitest using the in-memory channel in `web/test/fake-channel.ts`.
- **Keep dependencies out.** Prefer the standard library and platform APIs. A new
  dependency needs a reason in the pull request.
- **Match the surrounding code.** Comments explain why, not what. No generated
  boilerplate or restating the function name.
- **Strings go in both locales.** Add every user-facing string to `web/src/i18n/en.ts`
  and `fa.ts`; the i18n test fails if the keys or `{placeholders}` differ.
- **Schema changes are migrations.** Append a script to `migrations` in
  `internal/db/db.go`; never edit one that has shipped.
- **Docs travel with behavior.** Update the README and `.env.example` when you add or
  change configuration.
- **Security-relevant changes** need a line in the pull request on what attack they
  address. Report vulnerabilities privately; see [SECURITY.md](SECURITY.md).

## Commits and pull requests

Write commit subjects in the imperative ("Cap concurrent argon2 hashes"), and use the
body to explain why. Keep pull requests focused: one concern per PR is much easier to
review than a mixed one.

## Releases

Maintainers tag `vX.Y.Z` on `main`. The release workflow builds binaries for Linux,
macOS and Windows, publishes `SHA256SUMS`, and pushes a multi-arch image to GHCR.
Move the `Unreleased` entries in [CHANGELOG.md](CHANGELOG.md) under the new version first.
