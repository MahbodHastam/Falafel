# Changelog

All notable changes are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Security

- Browsers now authenticate each other's DTLS fingerprints with a random key carried in
  the share link's `#fragment`. A signaling server that tampers with the SDP can no
  longer intercept a transfer. Links without the fragment are refused.
- TURN credentials can be issued per room from `FALAFEL_TURN_SECRET` (coturn
  `use-auth-secret`). Static TURN credentials are still accepted but are visible to
  every visitor of `/api/config`.
- Rate limits use the last `X-Forwarded-For` entry instead of the first, which clients
  could forge, and group IPv6 addresses by /64.
- The docker compose file publishes the port on `127.0.0.1` only and runs the container
  read-only with no capabilities.
- At most four argon2id hashes run at once, so parallel requests cannot exhaust memory.
- Wrong-password attempts are counted atomically; concurrent attempts were undercounted.
- Share tokens no longer appear in server logs.
- `Content-Security-Policy` drops `unsafe-inline` styles and names the WebSocket origin;
  `Referrer-Policy` is now `no-referrer`.
- File names containing direction-override characters are rejected, and downloads are
  saved with Windows reserved names, trailing dots and overlong names defused.
- Owner secrets are compared in constant time.

### Added

- Flow control: the sender stays within 32 MiB of what the receiver has written.
- The sender's WebSocket reconnects after a network drop without ending the transfer.
- Database schema versioning through `PRAGMA user_version`.
- `FALAFEL_TURN_SECRET`, `-version` flag, release workflow (binaries, `SHA256SUMS`,
  multi-arch GHCR image), CodeQL, Dependabot, `govulncheck` and `pnpm audit` in CI.
- `SECURITY.md`, `CONTRIBUTING.md`, issue and pull request templates.
- Unit tests for the receiver, sink, channel authentication, schema migrations, and
  more server paths.

### Changed

- Files are read in 1 MiB blocks instead of one 64 KiB slice per await.
- Browsers without the File System Access API fold received chunks into Blobs every
  64 MiB instead of holding every `ArrayBuffer`.
- A receiver that leaves without transferring no longer marks the room as failed.
- The hub reads from SQLite outside its lock and forgets rooms with nobody connected.
- docker compose passes every `FALAFEL_*` setting through; before, most were ignored.
- `FALAFEL_BASE_URL` is validated as a URL.
- CI builds with the newest Go 1.26 patch release.

### Removed

- The undocumented `FALAFEL_TURN_URL` alias; use `FALAFEL_TURN_URLS`.
- The empty `app.tagline` string and its dead markup, which made the i18n test fail.
- Stray `// ponytail:` marker comments.
