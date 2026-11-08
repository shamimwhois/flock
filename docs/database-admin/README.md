# Database Admin — Planning & Design Pack

Status: **Planning baseline — implementation not started**  
Repository: Flock  
Branch: `feature/database-admin-panel-security-hardening`

This document set defines the product behavior, UI/UX, backend architecture, platform integration, security boundaries, file-by-file implementation responsibilities and validation plan for the Database Admin feature.

## Product decision

Flock exposes two entry points backed by a shared local administration runtime:

- **Global Database Admin** in the application's main navigation. It opens a generic Adminer login page. No site is required and no site's credentials are preselected or injected.
- **Per-site Database Admin Login** in every site's own details/sidebar. It opens the database connection belonging to that exact site, based on safe configuration discovery or a user-managed profile.

A per-site launch must never silently fall back to the global login or another site's connection. The global entry point must remain usable when no site exists or is selected.

## Documents

| Document | Purpose |
|---|---|
| [Product requirements](./01-product-requirements.md) | Scope, personas, workflows, states and acceptance criteria |
| [UI/UX design](./02-ui-ux-design.md) | Main navigation, global screen, site sidebar, interaction and visual states |
| [Architecture](./03-architecture.md) | Go daemon services, API boundaries, Adminer runtime and cross-platform responsibilities |
| [Code structure](./04-code-structure.md) | Proposed files/packages and file-by-file responsibilities |
| [API & data contracts](./05-api-and-data-contracts.md) | Endpoints, request/response models, sanitized metadata and errors |
| [Security threat model](./06-security-threat-model.md) | Secrets, ticket exchange, local listener, abuse cases and mitigations |
| [Testing & delivery plan](./07-testing-and-delivery.md) | Test matrix, implementation phases, CI gates and definition of done |

## Existing repository integration points inspected

- Go backend: `internal/api`, `internal/config`, `internal/catalog`, `internal/daemon`.
- macOS: Swift packages under `macos/Sources/FlockUI` and `macos/Sources/FlockKit`.
- Windows: `windows/Flock.Kit` and the existing Windows UI project(s).
- Linux: Rust/Tauri application under `linux/src`, with pane modules under `linux/src/panes`.
- Current service catalog: `internal/catalog`; service definitions already include database-related services. New Adminer tooling must be modeled separately from database server services.

These are integration points, not proof that the proposed files already exist. All paths in the code-structure document are proposed additions unless explicitly marked existing.

## Fixed product decisions

1. Adminer is the first/default administration tool because it supports the initial engine set in one lightweight interface.
2. Initial engines: MySQL, MariaDB, PostgreSQL and SQLite, subject to real integration tests.
3. phpMyAdmin and pgAdmin are optional later integrations, not mandatory bundled dependencies.
4. Adminer is not itself a database server; Flock must not confuse installing a database service with opening a database administration UI.
5. Database credentials are never placed in URLs, browser history, logs, telemetry, MCP output or ordinary status payloads.
6. Project configuration discovery is read-only. Never execute PHP project code or evaluate framework config files.
7. All administration traffic is loopback-only and excluded from public site sharing/tunnels.
8. Global launch and per-site launch are separate intents, even if they reuse the same local runtime.
9. No database, query or mutation API is exposed through the general Flock daemon API. The admin tool communicates directly with the database over its own constrained local session.
10. This pack is planning only. No Go, Swift, C# or Rust implementation is claimed complete.
