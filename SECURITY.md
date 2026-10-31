# Security policy

## Reporting a vulnerability

Please report vulnerabilities privately through
[GitHub security advisories](https://github.com/MahbodHastam/falafel/security/advisories/new).
Do not open a public issue for something exploitable.

Include what you found, how to reproduce it, and which version or commit you tested.
You can expect an acknowledgement within a few days. Fixes ship in a patch release,
and the advisory credits you unless you prefer otherwise.

## Supported versions

Only the latest release receives security fixes.

## Scope

In scope: the Go server (`cmd/`, `internal/`), the web client (`web/src`), and the
Docker and Caddy examples in this repository.

Particularly interesting:

- Anything that lets a party without the full share link (including its `#fragment`)
  obtain or alter a transferred file.
- Bypassing the password check, rate limits, owner-secret check, or signaling tickets.
- Server-side memory or CPU exhaustion from unauthenticated requests.
- Leaking share tokens, owner secrets, or link keys to logs or other origins.

Out of scope:

- Reports that rely on a malicious operator serving modified JavaScript. The
  [threat model](README.md#threat-model) states this limit.
- Findings that need a modified browser or access to the victim's device.
- Missing hardening on a deployment you configured differently from the documented setup.
