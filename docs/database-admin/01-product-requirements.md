# 01 — Product Requirements

## 1. Objective

Provide a safe, one-click database administration experience in Flock with two clearly different entry points: global administration and per-site administration.

## 2. Users and jobs-to-be-done

- Developer wants to open a database admin tool without manually finding a tool URL.
- Developer wants a site-specific button that opens the database configured for that site.
- Developer wants a global login page to connect to any authorized database, even without selecting a site.
- Developer wants to inspect the detected connection before opening it, without exposing the password.

## 3. Functional requirements

### FR-01 — Global navigation
- Add a persistent **Database Admin** item to the main/global navigation.
- It remains available with zero sites, no selected site, or a site without a detected DB.
- Clicking it opens the generic Adminer login page in the system default browser.
- It must not prefill or reuse credentials from the currently selected site.
- Global mode supports user-entered server, username and password in the admin tool.
- Show tool availability and a useful error if Adminer is not installed/verified or the local runtime cannot start.

### FR-02 — Per-site details
- Each site details/sidebar view contains a Database section next to Site Information.
- Show engine, database name/path, host and port where applicable, username, configuration source and connection state.
- Mask all passwords. Do not render full DSN/URL.
- Multiple detected connections are represented by a selector with stable connection labels.
- The primary action is **Database Admin Login** or an engine-aware label such as **Open PostgreSQL Admin**.
- Launch must use the selected connection ID scoped to the selected site.
- No database found: show **No database detected** and **Configure connection**.
- Ambiguous configuration: show a warning and require user selection.
- Failed connection: show a sanitized reason and allow retry/edit.
- Provide **Test Connection** and a stop/close action when the panel is running.

### FR-03 — Discovery
- First provider: Laravel-style dotenv keys and DATABASE_URL.
- Parse dotenv text conservatively; never execute PHP, Composer scripts, framework bootstraps or arbitrary project code.
- Support MySQL, MariaDB, PostgreSQL and SQLite.
- Support a manual connection profile for projects with other config formats.
- Discovery is read-only and must not change project files.
- Re-scan on explicit refresh and when the site configuration changes; avoid continuous expensive polling.

### FR-04 — Connection behavior
- Validate database connectivity before a site-specific launch, with bounded timeout.
- Database host may be local or remote. Non-loopback/private-network remote targets require explicit user approval; do not break normal local development usage.
- SQLite must resolve to an approved file path and reject traversal/symlink escape unless the user explicitly approves the resolved external path.
- Do not auto-create database users, change grants or alter database configuration.

### FR-05 — Runtime lifecycle
- Start a dedicated loopback-only admin listener on an OS-assigned ephemeral port.
- Wait until listener is ready before opening the browser.
- Reuse a running listener when safe; isolate sessions by launch intent and connection.
- Provide stop action, idle timeout and absolute session timeout.
- On app/daemon shutdown, invalidate tickets and sessions and stop owned runtime processes.

### FR-06 — Tools
- Adminer: first/default tool.
- phpMyAdmin: optional MySQL/MariaDB tool, later phase.
- pgAdmin: optional PostgreSQL tool, later phase.
- Tool binaries/assets must be pinned, integrity-verified and distributed through a trusted release/catalog path. Never execute an arbitrary downloaded PHP/EXE archive.

## 4. User stories and acceptance criteria

| ID | User story | Acceptance |
|---|---|---|
| US-01 | As a developer, I open global Database Admin | Opens generic Adminer login without a selected site |
| US-02 | As a developer, I open a site's DB | Correct site connection is selected and opened |
| US-03 | As a developer, I inspect DB settings | Metadata is visible; password remains masked |
| US-04 | As a developer, I have multiple DBs | I can select a connection before opening |
| US-05 | As a developer, no DB is detected | I see a configure action, not a broken launch |
| US-06 | As a developer, connection fails | I receive a sanitized actionable error |
| US-07 | As a developer, I stop the panel | Listener and sessions are invalidated |
| US-08 | As a developer, I have no site | Global entry still works |

## 5. Non-functional requirements

- Security: no credential leakage, strict loopback access, one-time launch tickets.
- Reliability: startup readiness check and bounded shutdown.
- Performance: no blocking UI while scanning or testing connections.
- Accessibility: keyboard-operable controls, clear focus, text labels for status icons.
- Portability: macOS, Windows and Linux behavior parity.
- Observability: lifecycle events may be logged, but never credentials, SQL values or result rows.

## 6. Out of scope

- Public or remote hosting of Adminer.
- Sharing database admin sessions through Flock site sharing.
- Remote database tunneling or SSH tunnel manager in the first release.
- Database creation, user provisioning, grants or migrations.
- SQL execution through Flock's general-purpose API/MCP tools.
- Running project-specific PHP config to infer connections.
