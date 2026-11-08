# Database Admin Panel — Product & Security Specification

Status: proposed implementation specification
Target: Flock native applications on macOS, Windows and Linux

## Goals
1. Add an Open database action to each site in the Flock panel and site details.
2. Detect a site's database connection from project configuration without modifying it.
3. Open a local browser-based database administration panel with one click.
4. Support MySQL, MariaDB, PostgreSQL and SQLite initially.
5. Offer Adminer as the lightweight default and optional phpMyAdmin / pgAdmin integrations.
6. Keep the database administration service local-only and independently stoppable.

## User experience
Every site's existing site sidebar/details sidebar must contain a dedicated **Database** section alongside the site's URL and project information. This is a per-site control, not only a global database page. The section displays the detected database engine, database name, host, port, database/admin username, configuration source and live connection status. Passwords are masked and are never rendered as plain text. Never expose raw connection URLs or DSNs.

### Per-site sidebar layout and interaction
- Keep the existing Site Information (site name, local URL, project path and site status) visible.
- Add a Database Information block directly in that same site's sidebar: engine badge, database name, host:port, username and connection status.
- Add a prominent **Database Admin Login** button in the Database block. The button launches the browser-based admin tool for that exact site's selected connection; it must not merely navigate to a generic Flock settings page.
- On click, Flock validates the selected connection, starts or reuses the local admin-panel service, issues a short-lived one-time launch ticket, waits until the listener is ready, and opens the system default browser. The browser then lands in the authenticated Adminer/phpMyAdmin/pgAdmin session for that connection.
- If multiple connections are detected, show a compact connection selector and make the selected connection explicit before launching. The button label may include the selected engine/tool, e.g. **Open PostgreSQL Admin**.
- If no database is detected, show **No database detected** and a safe **Configure connection** action. If detection is ambiguous or the connection test fails, explain the issue and allow the user to select/edit a local-only profile.
- Provide a secondary **Test Connection** action and a panel stop/close action while the panel is running.
- Never put passwords, full DSNs or credentials in the sidebar URL, browser URL, clipboard by default, logs, MCP output or telemetry.

### Expected click flow
1. User selects a site and sees its Site URL plus Database Information in the site's sidebar.
2. User clicks **Database Admin Login**.
3. Flock resolves that site's selected connection and performs a safe connection check.
4. Flock launches/reuses the loopback-only admin service and opens the browser.
5. The admin tool opens already associated with the selected database connection, without asking the user to paste credentials into a URL or exposing the password in the UI.

- Copy connection details copies host, port, database and username only by default.
- Multiple connections are shown as a selectable list.
- If configuration is missing or ambiguous, offer a manual profile without writing secrets into the project.

## Engine and project detection
- Laravel dotenv: DB_CONNECTION, DB_HOST, DB_PORT, DB_DATABASE, DB_USERNAME, DB_PASSWORD and DATABASE_URL.
- Parse dotenv syntax conservatively; do not execute PHP or evaluate Laravel config files.
- If config sources conflict, show ambiguity and let the user choose.
- Support PostgreSQL, MySQL, MariaDB and SQLite.
- Validate SQLite paths against the site path or an explicitly approved external path; resolve symlinks and prevent traversal.
- Do not assume a service is running just because a .env value exists.

## Browser panel architecture
- Bundle or fetch a pinned, checksum-verified Adminer release through Flock's signed package/catalog mechanism.
- Serve Adminer from a dedicated loopback-only listener on an ephemeral or reserved local port.
- Bind only to 127.0.0.1 and ::1 where supported; reject non-loopback peers at the handler boundary too.
- Never expose the admin panel through public sharing/tunnels or a site's normal reverse proxy.
- Open the default browser only after the listener is ready.
- Add Stop database panel and an idle timeout.
- Keep panel routes isolated from site document roots and site middleware.

## Secure one-time connection handoff
1. Native UI requests a launch ticket from the authenticated local Flock API.
2. Daemon creates a cryptographically random, single-use ticket with a maximum 60-second TTL.
3. Browser opens a clean loopback URL containing only the opaque ticket.
4. A same-origin POST exchanges the ticket for a short-lived server-side session; ticket consumption is atomic.
5. Password is delivered only over loopback POST/session flow, never embedded in HTML or JavaScript.
6. Set HttpOnly, SameSite=Strict, Secure when HTTPS is used, and path-scoped cookies. Send Cache-Control: no-store, Referrer-Policy: no-referrer and restrictive CSP.
7. Enforce CSRF/origin checks for state-changing operations and re-authenticate on expiry.
8. Never persist discovered passwords in Flock config, telemetry, crash reports or logs.

## Security controls
- Keep listener loopback-only and verify peer IP on every request.
- Require local API authentication to create launch tickets; do not reuse public share tokens.
- Ticket entropy at least 256 bits; constant-time comparison; one-time atomic consumption; 60-second TTL.
- Session idle timeout 15 minutes by default; absolute timeout 2 hours.
- Rate-limit ticket creation and failed login attempts.
- Non-loopback/private-network database endpoints require explicit user approval.
- Reject Host-header and DNS-rebinding attempts; validate Host against exact loopback origins.
- Apply strict request body limits and timeouts.
- Escape displayed project metadata and SQL errors.
- Do not log SQL values, passwords, connection URLs or result rows.
- Offer read-only mode; write operations require explicit user choice and least-privileged database grants.

## Admin tool choices
| Tool | Engines | Role |
|---|---|---|
| Adminer | MySQL, MariaDB, PostgreSQL, SQLite | Default lightweight unified panel |
| phpMyAdmin | MySQL, MariaDB | Optional dedicated MySQL/MariaDB experience |
| pgAdmin 4 | PostgreSQL | Optional advanced PostgreSQL administration |

Do not install all tools by default. Adminer is the first implementation target. phpMyAdmin and pgAdmin are optional catalog-managed integrations and must use the same local-only launch/session boundary. Do not claim engine compatibility until connection and CRUD tests pass.

## API contract (proposed)
- GET /v1/sites/{site}/database: sanitized detected metadata; never returns password or raw DSN.
- POST /v1/database/panel/tickets: accepts site and connection identifier; returns one-time ticket and local launch URL.
- POST /_panel/exchange: consumes ticket and establishes panel session; available only on dedicated loopback listener.
- POST /v1/database/connections/test: tests connection and returns sanitized status/error.
- POST /v1/database/panel/stop: stops listener and invalidates sessions.

All API routes require existing local API authentication. Browser exchange is the sole exception and accepts only a valid single-use ticket from a loopback peer.

## Acceptance criteria
- Each site shows detected database engine and sanitized connection details.
- One click opens the correct database without credentials in URL.
- Supports Laravel dotenv keys and DATABASE_URL.
- Multiple database connections are selectable.
- MySQL, MariaDB, PostgreSQL and SQLite have integration tests.
- Invalid, expired and replayed tickets are rejected.
- Non-loopback access, Host-header attacks, DNS rebinding, public-share access and CSRF attempts are rejected.
- Passwords and DSNs never appear in logs, API status, MCP responses or browser history.
- macOS, Windows and Linux UI/daemon support.
- Panel can be stopped and expires after inactivity.

## Delivery phases
1. Database config detection and sanitized site API.
2. Adminer package/catalog integration and loopback panel lifecycle.
3. One-time ticket/session exchange and browser launch.
4. macOS, Windows and Linux UI actions.
5. Optional phpMyAdmin and pgAdmin integrations.
6. Security regression tests, cross-platform CI and documentation.

## Out of scope
- Publicly hosting database administration.
- Exposing database credentials through MCP by default.
- Automatically creating database users or changing grants.
- Running arbitrary SQL through the Flock daemon API.
- Executing project PHP config or bootstrapping projects to discover connections.