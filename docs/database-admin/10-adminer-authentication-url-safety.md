# 10 — Adminer Authentication & URL-Safety Assessment

Status: **Upstream behavior reviewed; integration must use a Flock-maintained compatibility layer**
Date: 2026-11-08

## 1. Executive decision

Do **not** launch stock Adminer with a URL such as:

```text
/?pgsql=db-host&username=db-user&db=app_db
```

Even when the password is absent, this exposes database identity and connection metadata in browser history, address-bar state, copied links, access logs and potentially referrer data. It also violates the intended clean-URL handoff contract.

Do **not** assume that an Adminer plugin implementing only `credentials()` solves this. Upstream authentication checks `isset($_GET["username"])` and `get_password()` before calling `adminer()->credentials()`; the normal post-login redirect also builds a URL containing driver/server/username/database parameters.

**Decision:** ship a pinned Adminer build with a small, reviewed Flock compatibility patch (or an equivalent upstream-supported mechanism only if proven by an executable test). The browser URL must remain a clean path with no database identity or credentials. The patch must map a server-side Flock session context to Adminer's driver, server, username and database state internally, without trusting browser-supplied connection parameters.

## 2. Exact upstream version inspected

The current official Adminer download page lists **Adminer 6.3.0**, released 2026-11-03. The Packagist source metadata identifies the upstream source commit as:

- Version: `6.3.0`
- Upstream commit: `6f95564e80f24a2a024a9cb9aee40cfd0aca8eb1`
- Reviewed file: `adminer/include/auth.inc.php`
- Reviewed file blob SHA: `0fe8d1f4d511cbc81b8c72fa6c37dee1e0da029f`

This is an exact commit-pinned source inspection, not merely a review of the moving `main` branch. Before shipping, the artifact still needs its own downloaded-byte SHA-256, license/NOTICE verification and signed-manifest entry.

## 3. Upstream source observations

At the exact 6.3.0 commit:

- `auth.inc.php` processes login POST data and stores the submitted password using `set_password()` in the PHP session.
- It then calls `auth_url($vendor, $server, $username, $db)` when the login needs a redirect. This URL builder carries the driver/server/username/database identity.
- The authenticated request path is gated by `isset($_GET["username"])` and `is_string(get_password())`.
- Only after that URL-gated branch does Adminer call `adminer()->credentials()` and establish the DB connection.
- The same file's logout path clears Adminer session keys and redirects.
- Adminer's functions source scopes session state by driver, server and username from request state. The password is kept server-side, but connection identity still participates in URL/session routing.
- Adminer's developer documentation describes driver selection from URL parameters such as `pgsql=`, authentication when `username=` is present, and POST/CSRF handling for state-changing actions.
- The official plugin catalogue lists login plugins, but a plugin's existence is not proof that it satisfies Flock's clean-URL requirement.

Relevant upstream references:
- Exact 6.3.0 authentication source: https://github.com/vrana/adminer/blob/6f95564e80f24a2a024a9cb9aee40cfd0aca8eb1/adminer/include/auth.inc.php
- Exact 6.3.0 functions source: https://github.com/vrana/adminer/blob/6f95564e80f24a2a024a9cb9aee40cfd0aca8eb1/adminer/include/functions.inc.php
- Developer documentation: https://github.com/vrana/adminer/blob/6f95564e80f24a2a024a9cb9aee40cfd0aca8eb1/docs/developing.md
- Official plugin catalogue: https://www.adminer.org/en/plugins/
- Official download/version page: https://www.adminer.org/en/

## 4. Why a simple plugin is insufficient

A plugin's `credentials()` hook can supply the connection password/host, but upstream auth flow currently gates connection processing on a URL `username` parameter. The normal login redirect also reconstructs URL parameters from the selected driver, server, username and database.

Therefore:
- A credentials-only plugin does not eliminate URL identity parameters.
- Pre-populating PHP `$_GET` in a front controller is not sufficient by itself: Adminer may emit those values again in generated links and redirects.
- Rewriting URLs at the reverse-proxy layer is brittle and can break Adminer's navigation, forms, exports and redirects.
- Passing a ticket or opaque connection ID as a query parameter is not an acceptable substitute; tickets belong in a short-lived POST exchange, not in persistent browser URLs.

## 5. Required Flock compatibility behavior

The pinned Adminer integration must provide a server-side request context with these properties:

1. A clean browser path (for example `/`) contains no host, port, driver, username, database, DSN, password, ticket or connection identifier.
2. The browser first POSTs a one-time launch ticket to the dedicated loopback listener. The listener atomically consumes it and creates a random, HttpOnly, SameSite=Strict session cookie.
3. The server-side session stores a reference to a protected connection object. The secret-bearing connection object is never serialized into a browser cookie.
4. On each Adminer request, the compatibility layer resolves the session and supplies the selected connection context internally.
5. All generated Adminer links and redirects remain on the clean path. No fallback to stock query-string routing is permitted for site-scoped sessions.
6. Adminer authentication and authorization are still enforced. The patch must not silently bypass database authentication merely because the user has a valid Flock session; Flock session authorization and database server authentication are distinct controls.
7. Logout clears the Adminer connection state and Flock site-session binding. Session expiry, ticket replay, site deletion, connection profile changes and daemon restart revoke the binding.
8. Global mode stays separate: it opens the generic login form and does not receive any site's server-side connection context.
9. User-supplied Adminer URLs, query parameters or form fields cannot override the server-side site/connection scope.
10. Adminer state-changing POST requests retain upstream CSRF checks; Flock additionally validates exact Host, Origin and loopback peer.

## 5A. Additional exact source findings (2026-11-09)

The pinned source review was extended to the bootstrap and default Adminer class. The following details are confirmed at commit `6f95564e80f24a2a024a9cb9aee40cfd0aca8eb1`:

- `adminer/index.php` includes `include/bootstrap.inc.php` before dispatching page handlers.
- `bootstrap.inc.php` starts the Adminer PHP session, loads the driver implementations, then defines:
  - `SERVER` from `$_GET[DRIVER]`
  - `DB` from `$_GET["db"]`
  - `ME` from `relative_uri()` plus driver/server, username and database query fields when present.
- The same bootstrap includes `include/auth.inc.php` and `include/connect.inc.php` after those constants are defined. A compatibility layer added only after bootstrap cannot safely change these constants.
- The stock `Adminer::credentials()` returns `array(SERVER, $_GET["username"], get_password())`.
- The stock login form emits driver, server, username and database inputs. Hiding the form with CSS is not an authorization boundary.
- `auth_url()` explicitly reconstructs driver/server/username/database parameters. It must be replaced or bypassed in the pinned integration.
- `connect.inc.php` and page handlers use the `DB` constant and request parameters. It is not enough to patch only the first authentication redirect.
- Asset paths are generally rooted at `DIR`, while application navigation is built from `ME`; both must be checked in the final artifact.

### Consequence for patch placement

The compatibility hook must execute **before the stock bootstrap defines `SERVER`, `DB` and `ME`**, or the pinned bootstrap itself must be narrowly patched. The patch must establish a validated server-side request context before Adminer resolves its driver and connection.

For a site-scoped session, the request context must be authoritative and immutable for that request. Browser query/form values must not be able to replace the selected site or connection. The integration must also audit every helper that copies request parameters into hidden form fields or generated URLs; a clean initial address bar alone is insufficient.

For global mode, the integration must leave Adminer's generic login flow available while ensuring that login submission and subsequent authenticated navigation do not persist connection identity in URLs. The global and site-scoped modes therefore need explicit, separately tested behavior rather than one shared implicit fallback.

No source patch is committed by this documentation update. The next code change must include the pinned upstream source fixture, a minimal patch, and executable tests against that exact source revision.

## 6. Preferred implementation seam

Prefer a narrow maintained patch at the Adminer integration boundary over maintaining a broad fork:

- Pin upstream version 6.3.0 and commit `6f95564e80f24a2a024a9cb9aee40cfd0aca8eb1`.
- Record the exact downloaded artifact SHA-256 separately; the source commit is not the artifact checksum.
- Apply a small patch to authentication/context resolution and URL generation, with a machine-readable patch file and tests.
- Keep the patch isolated from database drivers, SQL rendering, export/import and the rest of Adminer.
- Add a compatibility test that scans every generated Location header, HTML link/form action and browser-visible URL for forbidden connection fields.
- If a future upstream release adds a stable supported server-side context API, replace the patch only after the same regression suite passes.

A plugin-only solution may be accepted only if a proof test demonstrates all of the above against the exact pinned Adminer release. Current source inspection does not establish that.

## 7. Test cases required before Phase 0 exit

- Global login renders with a clean URL and no inherited site context.
- Site launch POST exchanges a single-use ticket and redirects to a clean URL.
- Reload and navigation retain only the server-side connection scope.
- No generated link, redirect, form action, asset URL or referrer contains driver, server, username, database, DSN, password, ticket or profile ID.
- Attempted query/form overrides cannot switch the site, connection, driver, host, username or database.
- Cross-site session cookie, missing cookie, expired session, replayed ticket and invalid Origin are rejected.
- Logout revokes only the current scoped session and cannot leave a reusable connection context.
- Two sites open in separate browser profiles/sessions and cannot cross-read or cross-switch connections.
- Adminer concurrent requests and exports continue to work with session locking semantics.
- Global mode remains generic and never silently selects the last-used site.

## 8. Phase gate update

The exact 6.3.0 source review confirms a **material integration constraint**. The previously proposed generic one-time ticket exchange is still valid, but the Adminer handoff must not rely on stock query-string login or a credentials-only plugin.

Phase 0 remains open. No Adminer patch, runtime prototype or integration test has been executed in this pass. Before implementation, build the smallest compatibility patch against the pinned source, then run the clean-URL/session-isolation regression tests on the resulting artifact.


## 9. Adjacent token-input hardening (2026-11-09)

While the Adminer patch remains blocked on a full pinned-source fixture, the existing Go launch-ticket and browser-session primitives received a small independent hardening change:

- Both ticket consumption and session lookup/revocation now accept only canonical, unpadded base64url encodings of exactly 32 bytes.
- Decoding uses strict trailing-bit validation and verifies a round-trip re-encoding before hashing or touching stored state.
- Regression tests cover non-canonical trailing bits for both token types.

This prevents alternate textual encodings from being treated as valid opaque-token representations and keeps ticket/session parsing rules identical. It does **not** replace the outstanding Adminer clean-URL compatibility patch or the required runtime integration tests.


## 10. Request-boundary identity rejection

The Go loopback session boundary now rejects Adminer request URLs containing connection-identity fields before dispatching to the Adminer handler. The check is case-insensitive, rejects malformed query encoding, and covers the upstream driver keys ('mysql', 'pgsql', 'sqlite', 'mssql', 'oracle'), 'username', 'server', 'db', 'ns', and sensitive handoff fields such as 'ticket', 'password', 'connection_id' and 'profile_id'. It also rejects bracketed form-style aliases such as 'auth[username]'.

Ordinary Adminer navigation parameters such as 'table', 'select', 'sql', 'dump' and 'script' remain permitted. The guard is deliberately fail-closed: a stock Adminer login URL will be rejected rather than silently accepted.

This is **inbound request hardening only**. It does not rewrite or validate Adminer's generated HTML links, form actions, JavaScript navigation, or 'Location' headers. It is not the pinned Adminer compatibility patch and does not close Phase 0. The source-level patch and executable tests against the exact 6.3.0 source fixture remain mandatory.
