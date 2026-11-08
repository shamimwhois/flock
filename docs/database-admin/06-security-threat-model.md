# 06 — Security Threat Model

## 1. Assets

- Database passwords and usernames.
- Database host, port, database name and file paths.
- Adminer runtime assets and integrity metadata.
- Launch tickets and browser sessions.
- Site-to-connection ownership mapping.
- Local database contents and administrative operations.

## 2. Trust boundaries

1. Native UI ↔ authenticated Go daemon API.
2. Go daemon ↔ site project files during read-only discovery.
3. Browser ↔ dedicated loopback Adminer listener.
4. Adminer ↔ selected database endpoint.
5. Flock ↔ OS credential vault.
6. Flock updater/catalog ↔ downloaded Adminer assets.

## 3. Threats and required mitigations

| Threat | Required mitigation |
|---|---|
| Password in URL/history/referrer | Never place credentials in URL; one-time opaque ticket only |
| Ticket replay | 256-bit CSPRNG token, hash at rest, atomic single-use consume, <=60s TTL |
| Ticket theft by another local process | Loopback peer checks, short TTL, origin/host validation, one-time use; document local-user threat limits |
| DNS rebinding / Host header attack | Exact loopback Host allowlist, reject unexpected Host and Origin |
| Remote network access to panel | Bind only 127.0.0.1/::1; verify peer IP on every request |
| Public share accidentally exposes panel | Separate listener; never register admin route with share/proxy/tunnel |
| Cross-site connection mix-up | Enforce site_id + connection_id ownership on every operation |
| Secret leakage in API/status/MCP/logs | DTO allowlist; structured redaction; serialization tests |
| Malicious project dotenv values | Parse data only; never execute code or shell interpolation |
| SQLite path traversal/symlink escape | Canonicalize path, resolve symlinks, enforce allowed roots or explicit approval |
| SSRF via database host | Explicit user approval for remote/private-network targets; DNS/IP validation at connect time |
| Malicious Adminer download | Signed catalog metadata, pinned version, checksum verification, atomic install |
| CSRF against local panel | SameSite cookies, strict Origin/Referer validation, CSRF token for mutations |
| XSS via DB names/errors | Contextual escaping, restrictive CSP, avoid raw driver errors |
| Long-lived stolen browser session | HttpOnly cookie, idle + absolute expiry, revoke on stop, no persistent remember-me |
| Database destructive operation | Adminer native capabilities remain; offer read-only profile where supported and make write access explicit |

## 4. Ticket/session policy

- Ticket: minimum 32 bytes from OS CSPRNG; base64url encoded.
- Store only SHA-256/HMAC hash; compare in constant time where applicable.
- Scope ticket to intent, site ID (site mode), connection ID (site mode), tool, creation time and expiry.
- Consume atomically exactly once.
- Ticket expiry <= 60 seconds.
- Session: server-side random ID, HttpOnly, SameSite=Strict, Secure when HTTPS; path-scoped.
- Default idle timeout 15 minutes and absolute timeout 2 hours.
- Stop/restart revokes all active sessions and pending tickets.
- No password in cookie, ticket, URL, HTML or JavaScript.

## 5. Secret handling

- Project-discovered secrets are read only when needed for test/launch and are not persisted by discovery.
- Manually saved profiles use OS credential vault/keychain/credential manager. Flock config stores only a vault reference.
- If vault is unavailable, do not silently write plaintext secrets. Offer session-only profile or disable persistent save.
- Redact structured logs by field name and by known secret value.
- Do not return driver error strings; map to stable safe error codes.
- Do not expose DB passwords through MCP tools by default.

## 6. Local attacker assumptions

Loopback-only does not defend against every process running as the same OS user. Ticket entropy, origin checks, peer checks and short lifetime reduce risk but do not create a hard boundary between mutually untrusted processes under the same user account. Document this limitation. Never claim that loopback alone is sufficient authentication.

## 7. Security release gates

- Security tests for invalid/replayed/expired tickets.
- Host, Origin, CSRF, non-loopback and DNS-rebinding regression tests.
- Site/connection cross-scope tests.
- Secret serialization/log redaction tests.
- Asset tamper and downgrade tests.
- SQLite traversal and symlink tests.
- Remote DB approval tests.
- Cross-platform permission and credential-vault review.
