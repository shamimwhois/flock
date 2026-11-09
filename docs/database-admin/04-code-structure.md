# 04 — Code Structure and File-by-File Responsibilities

## 1. Ground rules

- This is a proposed implementation layout based on the repository structure inspected on the feature branch.
- Existing files are labeled **modify**; proposed new files are labeled **new**.
- Do not create duplicate API servers, duplicate daemon lifecycle managers or a second site registry.
- Keep core logic platform-neutral in Go. Platform clients should be thin API/UI adapters.
- Add code only after architecture and Adminer runtime feasibility are approved.

## 2. Go backend

| File | Status | Responsibility |
|---|---|---|
| `internal/databaseadmin/model.go` | New | Define Engine, ConnectionID, ConnectionProfile, SafeConnectionInfo, Source and launch intent. Keep secret fields private/unserializable. |
| `internal/databaseadmin/discover.go` | New | Discover candidate connections for a site; aggregate sources and ambiguity results. |
| `internal/databaseadmin/dotenv.go` | New | Parse dotenv syntax safely, including quotes/comments/escaped values without evaluating shell expressions. |
| `internal/databaseadmin/databaseurl.go` | New | Parse supported URL schemes and percent-encoded credentials; immediately separate secrets from safe metadata. |
| `internal/databaseadmin/sqlite.go` | New | Resolve SQLite paths, symlinks, relative paths and containment/approval policy. |
| `internal/databaseadmin/validate.go` | New | Validate required fields, supported engine and host/port rules. |
| `internal/databaseadmin/connector.go` | New | Perform bounded, context-cancelable connectivity checks for supported drivers. |
| `internal/databaseadmin/credentials.go` | New | Define credential vault interface and secret reference resolution. |
| `internal/databaseadmin/ticket.go` | New | Generate cryptographically secure one-time launch tickets; hash ticket at rest; atomic consume; expiry. |
| `internal/databaseadmin/session.go` | New | Create, validate, expire and revoke server-side panel sessions. |
| `internal/databaseadmin/panel.go` | New | Start/stop loopback listener, manage runtime state, open session binding and idle shutdown. |
| `internal/databaseadmin/assets.go` | New | Pin Adminer version, verify signed/checksummed assets, reject tampered/unknown artifacts. |
| `internal/databaseadmin/errors.go` | New | Stable error codes with safe user-facing messages; never wrap secret-bearing driver errors into API output. |
| `internal/api/databaseadminapi.go` | New | Register authenticated endpoints; validate site/connection scope; map service errors to DTOs. |
| `internal/daemon/databaseadmin.go` | New | Construct manager, connect config/catalog, start and stop manager with daemon lifecycle. |
| `internal/databaseadmin/*_test.go` | New | Unit tests for parsers, path safety, ticket/session, runtime and connectors. |
| `internal/api/databaseadminapi_test.go` | New | API auth, site scoping, sanitization and launch behavior. |
| `internal/daemon/databaseadmin_test.go` | New | Lifecycle, startup failure, cleanup and shutdown tests. |

### Existing Go files to inspect/modify only when needed

| Existing file | Expected change |
|---|---|
| `internal/api/api.go` | Register routes and inject service dependency using current routing conventions. |
| `internal/daemon/daemon.go` | Own manager lifecycle; ensure clean stop and no duplicate listener. |
| `internal/config/config.go` | Add only non-secret feature/runtime settings if required; never store DB passwords here. |
| `internal/catalog/catalog.go`, `database.go`, `kinds.go` | Add Adminer tool metadata only if catalog model supports executable web tools; do not classify Adminer as a DB server. |
| `go.mod`, `go.sum` | Add only required, maintained drivers/runtime dependencies after license and platform review. |

## 3. macOS Swift

Existing client areas: `macos/Sources/FlockKit` and `macos/Sources/FlockUI`.

| Proposed file | Responsibility |
|---|---|
| `FlockKit/DatabaseAdmin.swift` | Codable safe DTOs: database summary, connection candidate, test result, launch response. Never decode password fields. |
| `FlockKit/DatabaseAdminClient.swift` | Typed methods for list/test/global launch/site launch/stop endpoints. |
| `FlockUI/DatabaseAdminStore.swift` | Observable state, loading/error/selection and action coordination. |
| `FlockUI/DatabaseAdminSection.swift` | Reusable per-site Database sidebar section. |
| `FlockUI/DatabaseAdminSettingsView.swift` | Optional global tool status/settings only; global launch action remains in main navigation. |

Modify existing `FlockKit/Client.swift`, `FlockKit/Models.swift`, `FlockUI/PanelView.swift` and relevant site detail views only after reading their current composition. Use the existing URL-opening abstraction.

## 4. Windows client

Existing shared client area: `windows/Flock.Kit`; UI project and view structure must be confirmed before implementation.

| Proposed file | Responsibility |
|---|---|
| `Flock.Kit/DatabaseAdmin.cs` | Safe DTOs and enums. |
| `Flock.Kit/DatabaseAdminClient.cs` | Typed daemon API client methods. |
| UI view model (existing UI project) | Loading, selection, test and launch commands. |
| Site details view (existing UI project) | Render database section and global navigation item. |

Modify `Flock.Kit/FlockClient.cs` and `Flock.Kit/Models.cs` only if that matches existing client conventions. Do not guess UI file paths until the full Windows UI tree is reviewed.

## 5. Linux Rust client

Existing API and UI areas: `linux/src/api.rs`, `linux/src/model.rs`, `linux/src/ui.rs`, `linux/src/panes`.

| Proposed file | Responsibility |
|---|---|
| `linux/src/database_admin.rs` | API DTOs and request wrappers, or split into module if project conventions require. |
| `linux/src/panes/database_admin.rs` | Global admin pane/settings state if a dedicated pane is needed. |
| `linux/src/panes/sites.rs` | Add per-site Database block using existing site view model. |
| `linux/src/panes/mod.rs` | Register new pane only if needed. |

Modify `api.rs`, `model.rs`, `ui.rs` and `panes/sites.rs` following current Rust architecture; avoid duplicating HTTP client setup.

## 6. Test file layout

- Go parser tests adjacent to parser source.
- Go security tests adjacent to ticket/session/panel source.
- API tests in `internal/api`.
- Swift tests in existing `macos/Tests` package targets.
- Windows tests in existing `windows/Flock.Kit.Tests`.
- Linux tests in module tests and existing Rust test targets.

## 7. Dependency policy

Before adding any dependency:
1. Confirm it is actively maintained and supports target OS/architectures.
2. Review license and transitive dependencies.
3. Pin versions and update lockfiles.
4. Document why the standard library/current dependencies are insufficient.
5. Add tests and CI coverage for the dependency's failure modes.


## 8. Source-audited integration points (2026-11-08)

This section supersedes any assumption that the proposed file list above is already implemented. The following existing source files were fetched from the feature branch and inspected:

| Platform | Confirmed existing integration point | Reuse requirement |
|---|---|---|
| macOS | `macos/Sources/FlockUI/PanelView.swift` | Add a compact global action using the existing SwiftUI section/row/shortcut style; do not replace the panel. |
| macOS | `macos/Sources/FlockUI/AppModel.swift` | Extend the existing shared observable state and existing `FlockClient`; do not create a second polling/state owner. |
| macOS | `macos/Sources/FlockUI/WebView.swift` | Keep Adminer out of this HTML viewer; open the loopback URL in the system browser. |
| Linux | `linux/src/panes/sites.rs` | Add the per-site Database section to the existing site detail rendering path and preserve the existing stack/list/detail lifecycle. |
| Linux | `linux/src/panes/services.rs` | Do not model Adminer as a database server or add it to the DB service catalog. |
| Linux | `linux/src/panes/common.rs` | Reuse `PaneView`, `group`, `info_row`, `open_row`, `status_dot` and `report` conventions. |
| Windows | `windows/Flock.Tray/PanelWindow.xaml` | Reuse existing XAML resource styles, `InfoBar`, row spacing and binding patterns for global entry point. |
| Windows | `windows/Flock.Tray/PanelWindow.xaml.cs` | Preserve tray panel lifecycle/size/hide behavior; do not turn the flyout into the Adminer browser. |
| Windows | `windows/Flock.Tray/FlockWindow.xaml.cs` | Use existing native window chrome if a dedicated configuration/details window is genuinely needed. |

### Exact UI work still pending

- macOS: inspect the Settings/site detail composition and the client API models; identify the exact view that owns selected-site details before adding the database section.
- Linux: finish tracing the remainder of `SitesPane::render_detail`, site model/config data flow, app navigation actions and the HTTP client conventions.
- Windows: inspect the complete `Features`, `ViewModels`, `Controls`, `Flock.Kit` client/model and site detail composition; the tray panel alone does not establish the site's detail page.
- All platforms: add global entry point, site-scoped connection selector/metadata, test action, launch state, safe errors, empty state and accessibility using native conventions.

No platform UI feature code has been added in this pass. The source audit is partial and does not establish that the per-site page integration is complete.

## 9. Runtime platform implementation boundary

The shared Go service owns discovery, validation, connection checks, tickets, sessions, Adminer artifact verification and authorization. Platform clients must not implement their own database discovery or credential parsing.

- macOS/Linux runtime adapter: dedicated supervised PHP-FPM master/pool, private Unix socket, Adminer-only INI/session directory, plus an independently managed loopback HTTP listener.
- Windows runtime adapter: dedicated supervised PHP-CGI FastCGI worker pool on loopback, private PHPRC and PHP_INI_SCAN_DIR, plus the same independently managed loopback HTTP listener.
- Both adapters must use the existing managed PHP installation and supervisor lifecycle. Neither may borrow a site's pool, site PHP version, project INI, public Caddy routes or site environment.
- The daemon owns one platform-neutral manager interface; OS-specific process specs are isolated behind platform-specific files/build constraints. Do not fork the business logic by OS.
- A listener handle must be bound before returning a launch URL. Readiness requires both HTTP listener and FastCGI workers healthy. Shutdown/restart must stop both and revoke tickets/sessions.
- Platform adapters are not considered implemented until integration tests prove process startup, readiness, request concurrency, timeout, crash recovery, cleanup and no leaked processes on each supported OS.

