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

## 2. Upstream source observations

Reviewed upstream Adminer documentation and source pages:

- `docs/developing.md` describes driver selection from URL parameters such as `pgsql=`, authentication when `username=` is present, and POST/CSRF handling for state-changing actions.
- `adminer/include/auth.inc.php` checks `isset($_GET["username"])` before connecting. It then calls `adminer()->credentials()`, but only after that URL-gated branch has been entered.
- The same authentication file constructs a redirect using `auth_url($vendor, $server, $username, $db)` after login, so the normal flow continues to put connection identity in the URL.
- `adminer/include/functions.inc.php` stores the password in the PHP session (encrypted using the Adminer key cookie) but scopes session entries by driver, server and username values derived from request state.
- Adminer documents a SameSite cookie and CSRF token approach and describes rejecting cross-site POST requests using `Sec-Fetch-Site`. Flock must retain its own Origin/Host/peer checks; SameSite alone is not sufficient.
- The official plugin catalogue lists several login plugins, but the existence of a plugin is not proof that it supports Flock's no-connection-data-in-URL requirement.

Relevant upstream references:
- https://github.com/vrana/adminer/blob/main/docs/developing.md
- https://github.com/vrana/adminer/blob/main/adminer/include/auth.inc.php
- https://github.com/vrana/adminer/blob/main/adminer/include/functions.inc.php
- https://www.adminer.org/en/plugins/

These references point to upstream main and are used for design analysis. They are not a substitute for inspecting the exact pinned release source during implementation.

## 3. Why a simple plugin is insufficient

A plugin's `credentials()` hook can supply the connection password/host, but upstream auth flow currently gates connection processing on a URL `username` parameter. The normal login redirect also reconstructs URL parameters from the selected driver, server, username and database.

Therefore:
- A credentials-only plugin does not eliminate URL identity parameters.
- Pre-populating PHP `$_GET` in a front controller is not sufficient by itself: Adminer may emit those values again in generated links and redirects.
- Rewriting URLs at the reverse-proxy layer is brittle and can break Adminer's navigation, forms, exports and redirects.
- Passing a ticket or opaque connection ID as a query parameter is not an acceptable substitute; tickets belong in a short-lived POST exchange, not in persistent browser URLs.

## 4. Required Flock compatibility behavior

The pinned Adminer integration must provide a server-side request context with these properties:

1. A clean browser path (for example `/` or `/site/<opaque-session>`) contains no host, port, driver, username, database, DSN, password, ticket or connection identifier.
2. The browser first POSTs a one-time launch ticket to the dedicated loopback listener. The listener atomically consumes it and creates a random, HttpOnly, SameSite=Strict session cookie.
3. The server-side session stores a reference to a protected connection object. The secret-bearing connection object is never serialized into a browser cookie.
4. On each Adminer request, the compatibility layer resolves the session and supplies the selected connection context internally.
5. All generated Adminer links and redirects remain on the clean path. No fallback to stock query-string routing is permitted for site-scoped sessions.
6. Adminer authentication and authorization are still enforced. The patch must not silently bypass database authentication merely because the user has a valid Flock session; Flock session authorization and database server authentication are distinct controls.
7. Logout clears the Adminer connection state and Flock site-session binding. Session expiry, ticket replay, site deletion, connection profile changes and daemon restart revoke the binding.
8. Global mode stays separate: it opens the generic login form and does not receive any site's server-side connection context.
9. User-supplied Adminer URLs, query parameters or form fields cannot override the server-side site/connection scope.
10. Adminer state-changing POST requests retain upstream CSRF checks; Flock additionally validates exact Host, Origin and loopback peer.

## 5. Preferred implementation seam

Prefer a narrow maintained patch at the Adminer integration boundary over maintaining a broad fork:

- Pin the exact upstream Adminer source/release and record its immutable commit/tag and SHA-256.
- Apply a small patch to authentication/context resolution and URL generation, with a machine-readable patch file and tests.
- Keep the patch isolated from database drivers, SQL rendering, export/import and the rest of Adminer.
- Add a compatibility test that scans every generated Location header, HTML link/form action and browser-visible URL for forbidden connection fields.
- If a future upstream release adds a stable supported server-side context API, replace the patch only after the same regression suite passes.

A plugin-only solution may be accepted only if a proof test demonstrates all of the above against the exact pinned Adminer release. Current source inspection does not establish that.

## 6. Test cases required before Phase 0 exit

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

## 7. Phase gate update

The upstream source review found a **material integration constraint**. The previously proposed generic one-time ticket exchange is still valid, but the Adminer handoff must not rely on stock query-string login or a credentials-only plugin.

Phase 0 remains open. No Adminer patch, runtime prototype or integration test has been executed in this pass. Before implementation, pin the exact Adminer release, build the smallest compatibility patch, and run the clean-URL/session-isolation regression tests on that artifact.
