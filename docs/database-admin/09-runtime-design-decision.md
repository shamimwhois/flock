# 09 — Runtime Design Decision (Phase 0)

Status: **Architecture direction selected; runtime prototype and cross-platform validation remain open**
Date: 2026-11-08

## 1. Source verification added in this pass

The current feature branch source confirms:

- internal/php/php.go maintains a Flock-owned extension list including mysqli, pdo_mysql, pdo_pgsql, pgsql, pdo_sqlite and sqlite3.
- ExtensionsIni enables an extension only when its shared library exists in the managed PHP ext/ directory. The list is an allow-list, not proof that every build ships every extension.
- php.Extensions(root, minor) executes the managed PHP binary and enumerates extensions loaded by its Flock CLI INI. Adminer must probe using its own isolated INI and scan directory.
- On macOS/Linux, fpmSpec generates a supervised PHP-FPM process with a private Unix socket, restrictive socket mode, explicit INI path and controlled environment.
- On Windows, cgiPool starts six supervised php-cgi.exe FastCGI workers bound to loopback. Each worker handles one request at a time. The pool accepts additional environment values and pins PHPRC and PHP_INI_SCAN_DIR.
- Flock already has a supervisor abstraction for long-running processes. Adminer must own separate specs and lifecycle rather than borrowing a site's normal PHP pool.

Source files reviewed:
- internal/php/php.go (extension loading, FPM generation, Windows CGI pool, extension probing)
- internal/php/manager.go (managed PHP installation lifecycle)
- internal/daemon/daemon.go (PHP manager wiring and site pool lifecycle)
- internal/web/web.go (normal site FastCGI routing)

## 2. Runtime decision

**Selected implementation direction: dedicated Adminer web listener + dedicated managed-PHP FastCGI execution context.**

Platform mapping:

| Platform | Adminer PHP execution |
|---|---|
| macOS / Linux | Dedicated PHP-FPM master/pool and private Unix socket |
| Windows | Dedicated PHP-CGI worker pool on OS-assigned loopback ports |

The Adminer HTTP listener must be separate from the normal site Caddy server configuration and bound only to 127.0.0.1. It must not be registered as a project site, reverse-proxy target, tunnel/share target or MCP resource.

Why:
- Reuses Flock's existing PHP binaries, FastCGI process model and supervisor.
- Avoids a second PHP runtime and avoids running arbitrary project PHP configuration.
- Provides a multi-worker request model, unlike the PHP CLI built-in server's default single-threaded mode.
- Keeps Adminer process lifecycle independent from site lifecycle and selected PHP versions.

The exact listener implementation (dedicated Caddy app/server versus a narrowly scoped Go HTTP-to-FastCGI bridge) is not yet frozen. Prefer a dedicated Caddy server if it can be started/stopped independently without sharing the public site route table. Do not route Adminer through the existing site server merely to reuse its process.

## 3. Isolated PHP configuration requirements

Create a private Adminer runtime directory, private INI and private session directory. Never point Adminer at a site's INI, .user.ini, project document root, project auto-prepend file or project environment.

Pin:
- PHPRC to the generated Adminer-only INI.
- PHP_INI_SCAN_DIR to an Adminer-only scan directory.
- extension_dir to the selected managed PHP version's extension directory.
- session.save_path to a private directory with owner-only permissions.
- open_basedir to the Adminer artifact, required runtime files, private session directory and explicitly required temporary directory, after platform verification.
- display_errors=Off, log_errors=On, expose_php=Off, allow_url_include=Off, cgi.fix_pathinfo=0.
- No project-controlled auto_prepend_file, auto_append_file, .user.ini or inherited project environment.

Do not blindly inherit Flock's ordinary development defaults or a user's editable PHP INI. Generate the Adminer INI from a strict allow-list.

## 4. Driver capability policy

At startup, execute the selected managed PHP binary with the Adminer-only INI and report only booleans for required capabilities. Verify:
- MySQL/MariaDB: mysqli and/or pdo_mysql according to the pinned Adminer build's actual driver path.
- PostgreSQL: pgsql and/or pdo_pgsql.
- SQLite: sqlite3 and/or pdo_sqlite.
- Sessions and all required core extensions for the pinned Adminer version.

Do not claim an engine is supported just because its extension name appears in the source allow-list. The actual installed artifact and runtime probe are authoritative. Missing drivers should disable only the affected engine and provide a clear repair/install path; never silently switch to system PHP.

## 5. Process and listener lifecycle

The Adminer manager should:
1. Select a compatible installed Flock PHP minor independently of the active site's PHP version.
2. Verify the pinned Adminer artifact and runtime capability report.
3. Allocate an ephemeral loopback listener and record the exact bound address.
4. Start the private PHP FastCGI pool and dedicated HTTP listener under the supervisor.
5. Wait for both listener and FastCGI workers to become ready before returning a browser launch URL.
6. Stop both components on Flock shutdown, feature disable, runtime replacement or fatal health failure.
7. Revoke outstanding tickets and sessions when the runtime is stopped or restarted.

Bind the listener first and retain the listener handle to avoid a port-allocation race. Do not accept a listener address supplied by an untrusted caller.

## 6. Remaining proof gates (not yet passed)

This is a source-backed architecture decision, not a runtime prototype report. The following remain mandatory before Phase 0 exit:

1. Build and run a dedicated FPM configuration on macOS/Linux with an Adminer-only INI and confirm clean shutdown/restart.
2. Build and run a separate Windows CGI pool using private PHPRC and scan dir; verify worker supervision and clean shutdown.
3. Prove the dedicated HTTP listener is loopback-only, independent from the public Caddy route table and excluded from sharing/tunnels/MCP.
4. Test actual extension availability on every supported platform/build channel; the source list alone is insufficient.
5. Inspect the exact pinned Adminer release's login/session code and prove a site-scoped connection can remain server-side without credentials in URL, HTML, JavaScript or browser-visible storage.
6. Confirm artifact publication through the signed manifest or a signed Flock release, including exact license and NOTICE files.
7. Exercise concurrent requests, long exports, uploads, timeouts, browser refresh, ticket replay, Host-header spoofing, DNS-rebinding-style hostnames and process restart.

## 7. Phase gate

**Phase 0 remains open.** No application feature code has been added, no runtime prototype has been executed, and no application tests or CI checks have been run in this pass. Feature implementation must not start until the remaining gates are recorded with platform-specific evidence and the listener transport is frozen.


## 8. Platform-specific execution cases and acceptance criteria

The source review establishes that the host Flock PHP managers can support the proposed execution models. It does **not** establish that a dedicated Adminer runtime already exists. The implementation must preserve the following distinct OS behavior.

### macOS and Linux — dedicated PHP-FPM

1. Select a compatible Flock-managed PHP minor independently of the selected site's PHP.
2. Generate a private Adminer-only FPM configuration and INI; use a private Unix socket under the protected Flock runtime directory.
3. Run a dedicated FPM master/pool under the existing supervisor. Do not attach Adminer to a site's FPM pool.
4. Create a private session directory with owner-only permissions and an isolated temporary directory.
5. Start a dedicated HTTP listener bound explicitly to IPv4 loopback. The listener must not be part of the normal public Caddy site route table.
6. Verify socket ownership/mode, worker identity, listener address, readiness, restart and shutdown behavior.
7. Confirm macOS launch-agent shutdown and Linux daemon/service shutdown both clean up the listener, FPM master/workers, tickets and sessions.

### Windows — dedicated PHP-CGI FastCGI pool

1. Select a compatible Flock-managed PHP minor independently of any site's PHP selection.
2. Generate private Adminer PHPRC and PHP_INI_SCAN_DIR locations; do not inherit project or user PHP configuration.
3. Start a dedicated set of supervised `php-cgi.exe` workers, bound only to loopback using OS-assigned ports. Do not reuse the six-worker site pool or its environment.
4. Use a private session/temp directory with restrictive Windows ACLs; verify the effective worker identity and file access.
5. Start a separate loopback HTTP listener and verify it cannot be reached through public site routes, sharing or tunnels.
6. Verify worker replacement after crash, bounded concurrency, request timeout, clean stop and file-unlock behavior during PHP package update/replacement.
7. On daemon shutdown/restart, stop every Adminer worker/listener and revoke all tickets/sessions.

### Shared acceptance matrix

| Proof | macOS | Linux | Windows |
|---|---|---|---|
| Dedicated managed PHP runtime | Not run | Not run | Not run |
| Private INI and extension probe | Not run | Not run | Not run |
| Private session storage permissions | Not run | Not run | Not run |
| Loopback-only HTTP listener | Not run | Not run | Not run |
| FastCGI worker readiness/concurrency | Not run | Not run | Not run |
| Crash/restart/clean shutdown | Not run | Not run | Not run |
| Public Caddy/share/tunnel isolation | Not run | Not run | Not run |
| Adminer auth URL-safety integration | Not run | Not run | Not run |

### Phase status

Architecture mapping is documented; runtime implementation and all platform execution proofs remain **not started / not verified**. Do not mark macOS, Linux or Windows runtime as passed until CI or reproducible platform test evidence is attached. A source-level presence of PHP-FPM/php-cgi support is not a runtime pass.
