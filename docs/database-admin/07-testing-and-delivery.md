# 07 — Testing and Delivery Plan

## 1. Test layers

### Unit tests
- dotenv parsing: quoted values, comments, escaped characters, duplicate keys, malformed values, empty values.
- DATABASE_URL parsing: percent encoding, IPv6 hosts, invalid schemes, missing components, conflicting dotenv values.
- Engine mapping: MySQL, MariaDB, PostgreSQL and SQLite.
- SQLite path: relative path, absolute path, traversal, symlink escape, missing file and approved external path.
- Ticket/session: entropy, expiry, atomic consume, replay, revocation and concurrency.
- Error mapping and secret redaction.

### API tests
- Auth required for all management endpoints.
- Global launch works with zero sites.
- Global launch never selects a site's connection.
- Site launch requires valid site and connection IDs.
- Cross-site connection ID is rejected.
- Responses contain no password, DSN or vault reference.
- Duplicate launch requests do not create unbounded listeners.
- Stop revokes tickets/sessions.

### Integration tests
- MySQL and MariaDB test containers.
- PostgreSQL test container.
- SQLite temporary database.
- Adminer loads from the verified artifact.
- Global login form works.
- Site-scoped session reaches only the selected DB.
- Browser exchange succeeds only once.
- Listener startup and shutdown work on supported OSes.

### Security regression tests
- Non-loopback peer rejected.
- Host header and Origin spoof rejected.
- DNS-rebinding host rejected.
- CSRF mutation rejected.
- Ticket expired/replayed/guessed rejected.
- Secret does not appear in logs, status, API response, panic report or URL.
- Public share and normal site proxy cannot reach panel.
- SQLite symlink/traversal rejected.
- Remote/private DB target requires approval.
- Tampered Adminer artifact rejected.

### Native client tests
- Global nav item visible with no sites.
- Per-site Database section visible for each site.
- Empty, ambiguous, failed, connected, starting and ready states render correctly.
- Browser opens only after successful launch response.
- Browser launch failure is recoverable.
- All controls are keyboard and accessibility accessible.

## 2. Platform matrix

| Platform | Go daemon/API | UI integration | Browser launch | Credential vault |
|---|---|---|---|---|
| macOS | Required | SwiftUI + FlockKit | NSWorkspace/openURL existing abstraction | Keychain |
| Windows | Required | Existing .NET UI + Flock.Kit | ShellExecute/default browser abstraction | Windows Credential Manager / DPAPI |
| Linux | Required | Rust/Tauri UI | xdg-desktop-portal or existing opener | Secret Service/libsecret where available |

Credential vault behavior must be explicitly tested on each supported desktop environment. Session-only credentials are the fallback if secure storage is unavailable.

## 3. Delivery phases

### Phase 0 — Design freeze
- Review all docs in this directory.
- Inspect current API, daemon, site model, platform UI patterns and catalog.
- Prototype Adminer packaging/runtime and confirm license, artifact source and PHP execution model.
- Finalize open questions and threat model.

### Phase 1 — Core models and discovery
- Implement normalized types and safe DTOs.
- Implement dotenv/DATABASE_URL discovery and SQLite validation.
- Add parser, ambiguity, and secret-sanitization tests.
- No browser launch yet.

### Phase 2 — Connectivity and credential handling
- Implement DB connectors with strict timeouts.
- Implement credential vault abstraction and session-only behavior.
- Add integration tests for supported engines.

### Phase 3 — Adminer runtime
- Verify and package Adminer artifact.
- Implement loopback listener and lifecycle.
- Implement global generic login first.
- Add runtime and artifact integrity tests.

### Phase 4 — Secure site launch
- Implement one-time ticket and server-side session exchange.
- Bind site/connection identity to session.
- Add all replay, CSRF, Host, peer and cross-site tests.

### Phase 5 — Native UI
- Add global nav item on macOS, Windows and Linux.
- Add per-site Database sidebar section.
- Add selector, test, launch and stop states.
- Confirm behavior parity.

### Phase 6 — Hardening and release
- Run full Go test suite and race tests where supported.
- Run platform test suites and CI.
- Perform manual browser flows on all OSes.
- Review third-party license notices and update docs.
- Only then mark PR ready for review.

## 4. CI gates

- `go test ./...`
- `go test -race ./internal/databaseadmin ./internal/api ./internal/daemon` on supported CI runners.
- `go vet ./...`
- Existing lint/static-analysis jobs.
- Swift package tests/build on macOS runner.
- Windows .NET build/tests on Windows runner.
- Linux Rust format, clippy and tests.
- Integration test jobs for supported database engines.
- Secret scanning and dependency vulnerability scanning.

Exact command availability must follow the repository's current CI workflow; add jobs without weakening existing gates.

## 5. Definition of done

- Global nav opens generic Adminer login with no site dependency.
- Every site's sidebar opens that site's selected DB only.
- No cross-site fallback or credential leakage.
- Supported engines pass real integration tests.
- Adminer asset integrity and licensing are documented.
- Listener is loopback-only and unavailable to public share/tunnel.
- Ticket/session security tests pass.
- macOS, Windows and Linux UI parity is verified.
- Full existing CI remains green.
- Documentation, release notes and third-party notices are updated.
