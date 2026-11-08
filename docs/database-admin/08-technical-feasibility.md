# 08 — Technical Feasibility & Architecture Decision Record

Status: **Phase 0 investigation — proposed direction, runtime prototype still required**  
Date: 2026-11-08

## 1. Evidence from current Flock repository

The following are confirmed from the feature branch source:

- Flock already manages PHP versions side by side through `internal/php`.
- macOS/Linux PHP packages include PHP CLI and PHP-FPM; Windows packages include `php.exe` and `php-cgi.exe`.
- Flock already has a signed Ed25519 binaries manifest with per-platform package hashes and anti-rollback serial checks in `internal/binaries`.
- Flock already embeds Caddy and routes normal sites to PHP-FPM sockets on macOS/Linux or loopback PHP-CGI workers on Windows.
- The existing site web server is deliberately configured around site domains and sharing. The Adminer endpoint must not be added as an ordinary project site or public share.
- The current PHP extension discovery code recognizes `pdo_mysql`, `pdo_pgsql` and `pdo_sqlite` as project requirements, but that does **not** prove every installed Flock PHP build has all three extensions enabled.
- The service catalog manages database servers. Adminer is a web application/tool and must remain a separate component, not a database service kind.

Relevant existing integration points:
- `internal/php/php.go`, `internal/php/manager.go`
- `internal/phpext/phpext.go`
- `internal/binaries/binaries.go`
- `internal/web/web.go`
- `internal/daemon/daemon.go`
- `internal/catalog/catalog.go`

## 2. Adminer upstream facts

As checked on 2026-11-08, the official Adminer site lists version **6.3.0**, released 2026-11-03. It offers a single PHP distribution and supports MySQL, MariaDB, PostgreSQL and SQLite (SQLite support is listed separately in its requirements/download details). Adminer requires PHP with sessions enabled and is distributed under Apache License or GPL 2. Official site: https://www.adminer.org/en/

Important:
- Adminer is a PHP application, not a database server.
- The official site also lists platform-native Adminer executables that include PHP and open in their own application window. Those are not the desired browser-based Flock UX, so they are not the default integration choice.
- Do not use the mutable `latest.php` URL in production installation logic. Pin an exact version and verify its integrity.
- Adminer itself supports powerful destructive operations and arbitrary SQL. Flock's local-only boundary and credential scope are therefore critical; do not assume the Adminer login form alone secures the whole feature.

## 3. Decision A — PHP runtime

### Proposed direction

Reuse a Flock-managed PHP runtime rather than shipping a second PHP runtime solely for Adminer.

- Select a Flock-installed PHP minor that satisfies Adminer's requirements.
- Run Adminer outside any project document root.
- Use a dedicated PHP execution context/pool with only required database extensions and sessions enabled.
- Never load the site's `php.ini`, project `.user.ini`, project auto-prepend scripts or project PHP code for the Adminer runtime.
- Do not assume the selected site's PHP version is installed or suitable; global Adminer must still work when no site exists.
- If no suitable PHP runtime exists, show a clear install/repair action rather than silently using a system PHP from PATH.

### Runtime transport prototype required

Two viable implementation paths need a small prototype before final selection:

1. **Dedicated Caddy + FastCGI**: isolated Caddy listener bound to loopback and a dedicated PHP-FPM pool on macOS/Linux; Windows uses a dedicated PHP-CGI/FastCGI worker pool. This best matches the existing Flock architecture and gives bounded request handling, but Windows parity and process lifecycle need validation.
2. **PHP CLI built-in server**: start the Flock-managed `php -S 127.0.0.1:<ephemeral>` with an isolated INI and Adminer-only document root. This is simpler for a prototype, but PHP's built-in server is single-threaded by default and must not be accepted as the production design without concurrency, request timeout, upload/export and process-isolation tests.

Do not expose Adminer through the existing general site Caddy route table or the share manager. A dedicated listener/runtime must be separately owned and stopped by the database-admin manager.

## 4. Decision B — Adminer asset distribution

### Proposed direction

Publish Adminer as a pinned artifact through Flock's existing signed binaries-manifest trust model, or embed a pinned artifact in a Flock release if the separate manifest/catalog route proves disproportionate.

Preferred package metadata:
- package ID: `adminer`
- exact upstream version (never floating `latest`)
- upstream source URL and license metadata
- SHA-256 digest
- release timestamp
- supported platforms/architectures (asset itself may be platform-independent)
- explicit yanked/withdrawn state and reason

Install process must:
1. Verify the signed manifest and anti-rollback policy using existing Flock binaries infrastructure.
2. Download with size/time limits.
3. Verify exact SHA-256 before extraction/use.
4. Install atomically into Flock's managed data directory, outside project roots.
5. Preserve license and NOTICE information.
6. Refuse missing/invalid digest, unexpected file type, path traversal archive entries or yanked release.

Do not download an unverified PHP file at runtime, execute a mutable upstream URL, or let an arbitrary local PHP file become the admin panel.

Open question: whether the existing binaries-manifest publisher can publish a platform-neutral PHP artifact cleanly. If not, embedding the pinned PHP artifact in a tagged Flock release is safer than inventing a second unsigned update channel.

## 5. Decision C — PHP extensions and engines

The Adminer runtime must perform a startup capability check using its own PHP binary/INI and report enabled drivers:
- MySQL/MariaDB: `mysqli` or `pdo_mysql` as required by the Adminer build.
- PostgreSQL: `pgsql` and/or `pdo_pgsql` as required.
- SQLite: `sqlite3` and/or `pdo_sqlite` as required.
- PHP sessions enabled and writable private session directory.
- Required core extensions (e.g. OpenSSL, JSON, mbstring) verified against the exact Adminer build requirements.

Do not infer support solely from the database server being installed. A driver can be missing even when the DB server runs.

The runtime capability report is sanitized and contains extension names/booleans only. It must never include INI contents or environment variables.

## 6. Decision D — Listener and browser security

- Allocate an OS-assigned ephemeral TCP port by binding explicitly to loopback.
- Prefer a single IPv4 loopback listener initially (`127.0.0.1`) until IPv6 dual-stack behavior is covered by tests; do not bind wildcard interfaces.
- Validate the exact Host header against the bound loopback address and selected port.
- Validate peer address on every request.
- Never expose listener through Caddy's public site routes, tunnel/share manager, reverse proxy or MCP.
- Use an unpredictable one-time ticket exchange, then a server-side session cookie.
- Adminer must not receive DB credentials in query strings, fragment, URL path, browser history, referrer, or client-side JavaScript.
- Test that Adminer does not unexpectedly persist connection secrets in cookies or generated links for the selected version. If it does, adapt the integration or reject that version.

## 7. Decision E — Credential handling

### Site-discovered connection
- Read the project dotenv file as data only.
- Keep the discovered password in memory only for the connection handoff.
- Never persist it into Flock config or a project file.
- Adminer may need credentials for subsequent requests; keep the connection context server-side and validate how Adminer expects it to be maintained. Avoid injecting the password into HTML or JS.

### Global connection
- User enters credentials in the Adminer login form over loopback.
- No site connection is automatically selected.
- Do not prefill with the last-used site's credentials.

### Manual profiles
- Persist passwords only through OS secure storage (Keychain, Windows Credential Manager/DPAPI, Linux Secret Service).
- If secure vault is unavailable, support session-only use and do not store plaintext in config.

## 8. Feasibility conclusion

**Feasible in principle** because Flock already manages PHP and has a signed package distribution system. The most maintainable route is to reuse Flock's PHP builds and create an isolated Adminer execution context.

However, this is not yet an implementation-ready final architecture. The following proof-of-concept gates remain:
1. Confirm all four DB drivers can be enabled across macOS, Windows and Linux PHP builds, or document platform-specific gaps.
2. Prototype dedicated Caddy/FastCGI on macOS/Linux and Windows.
3. Compare built-in PHP server fallback under concurrent browser requests, exports and uploads.
4. Confirm Adminer 6.3.0 login/session behavior supports a safe server-side preselected connection without credentials in URLs or HTML/JS.
5. Confirm Adminer license/NOTICE packaging obligations with the Flock license maintainers.
6. Confirm artifact publication and signature flow in the existing binaries repository.

## 9. Phase 0 exit criteria

Do not start feature implementation until all six proof-of-concept gates above have a documented result and a selected runtime design. Once validated, update Architecture (03) and Code Structure (04) to reflect the final decisions.
