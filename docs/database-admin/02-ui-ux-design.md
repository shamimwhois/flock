# 02 — UI/UX Design Specification

## 1. Navigation model

Database Admin has a global entry point and a per-site entry point. They are not interchangeable.

```text
Flock Main Navigation
├── Overview
├── Sites
│   ├── Site A
│   │   └── Site Details Sidebar
│   │       ├── Site Information
│   │       ├── Database
│   │       │   ├── Connection selector
│   │       │   ├── Database details
│   │       │   ├── Test Connection
│   │       │   └── Database Admin Login
│   │       └── ...
│   └── Site B
└── Database Admin  (GLOBAL)
```

## 2. Global navigation item

**Label:** Database Admin  
**Icon concept:** database cylinder / server stack  
**Placement:** main application navigation, not nested under a specific site.

Click behavior:
1. UI calls the authenticated local daemon to request a global launch.
2. Daemon starts or reuses the verified Adminer runtime.
3. Daemon creates a global-intent one-time launch ticket.
4. UI asks the OS to open the returned loopback URL.
5. Browser displays Adminer generic login form.

Global screen behavior:
- No site selector is required.
- No site credentials are prefilled.
- The user manually enters database system, server, username and password in Adminer.
- If Adminer is unavailable, show a non-blocking explanation with Retry/Install or repair action, depending on trusted package availability.
- Global launch does not display a list of discovered site passwords or DSNs.

## 3. Per-site sidebar layout

```text
┌──────────────────────────────────────┐
│ SITE: storefront                     │
│ ● Running                            │
│                                      │
│ SITE INFORMATION                     │
│ URL       http://storefront.test     │
│ Path      ~/Projects/storefront      │
│                                      │
│ DATABASE                             │
│ [MySQL] [● Connected]                │
│                                      │
│ Connection                           │
│ [ Primary connection            v ]  │
│                                      │
│ Database  storefront                 │
│ Host      127.0.0.1                  │
│ Port      3306                       │
│ Username  storefront_user            │
│ Password  •••••••••••••              │
│ Source    .env                       │
│                                      │
│ [ Test Connection ]                  │
│ [ Database Admin Login  ↗ ]         │
│                                      │
│ Last checked: just now               │
└──────────────────────────────────────┘
```

Notes:
- The drawing is a wireframe, not a screenshot or final pixel design.
- Hide password value completely by default; a masked placeholder may indicate that a credential exists. No reveal button in the initial release.
- Host/port are omitted or replaced with “File database” for SQLite.
- SQLite shows the canonical resolved path, with path truncation and copy affordance if appropriate.
- Username is visible because it is useful metadata; it is not a secret equivalent to the password.
- The connection selector appears only when multiple profiles are available.
- Primary action is visually dominant; Test Connection is secondary.

## 4. State designs

| State | UI presentation | Primary action |
|---|---|---|
| Detected + connected | Green/neutral connected indicator and metadata | Database Admin Login |
| Detected + not tested | Neutral “Not checked” state | Test Connection |
| Connection failed | Warning and sanitized reason | Retry / Edit connection |
| No DB detected | “No database detected” | Configure connection |
| Ambiguous config | Warning + source/connection selector | Select connection |
| Panel starting | Progress state, prevent duplicate clicks | Cancel if safe |
| Panel ready | “Admin panel running” and last launch info | Open Admin / Stop |
| Adminer unavailable | Tool unavailable explanation | Retry / trusted install |
| Session expired | Browser asks for a new launch | Reopen from Flock |

## 5. Interaction rules

### Site-specific launch
- Disable duplicate launch while request is pending.
- Show progress: “Starting database admin…”.
- If connection is unreachable, do not silently launch with another profile.
- On success, open browser only after the local listener is ready.
- If OS browser launch fails, display a copyable loopback URL only while its ticket remains valid; never copy credentials.
- If the ticket expires, request a new ticket rather than reusing the old one.

### Global launch
- Does not require a site or connection selection.
- Opens generic Adminer login.
- Must not auto-login with whichever site was last selected.

### Connection configuration
- Manual profile editor fields: display name, engine, host/socket/path, port, database, username, password, TLS mode and optional read-only preference.
- Password input uses secure platform control and is never echoed back by API.
- Save secrets only in the OS credential vault/keychain/credential manager; store only a secret reference in Flock config.
- Clearly label whether a profile is project-discovered or manually managed.

## 6. Visual language

- Follow existing Flock native design tokens and platform conventions.
- Use compact grouped information, not a separate dashboard page for every site.
- Keep Global Database Admin discoverable in main navigation.
- Use consistent engine labels/icons and accessible text status.
- Avoid color-only status communication; include text.
- No decorative chart or heavy analytics UI is needed for the first release.

## 7. Accessibility and responsive behavior

- All controls must be keyboard reachable and have explicit accessible labels.
- Connection selector and primary action must work with screen readers.
- Long paths and database names truncate with full value available through accessible tooltip/copy.
- Sidebar must scroll naturally on small windows and not force horizontal scrolling.
- Loading, empty, warning and error states must be announced to assistive technology.


## 8. Existing Flock UI baseline — source audit

The initial wireframes above are requirements, not permission to introduce a new visual system. The implementation must reuse each platform's current navigation, spacing, typography, status, error and action patterns.

### macOS (confirmed)
- The menu-bar panel is a compact SwiftUI `VStack` with `SectionTitle`, `ServiceRow`, status/error banners, and existing shortcuts in `macos/Sources/FlockUI/PanelView.swift`.
- Shared daemon state and polling live in `AppModel`; API access is through its existing `FlockClient`.
- `WebView` is an HTML viewer with non-persistent WebKit storage and remote-content blocking. It is not a suitable place to embed Adminer; the browser must be opened externally through the existing OS URL-opening convention.
- Integration rule: add a compact Database Admin shortcut/section using existing SwiftUI rows and app-model/client patterns. Do not replace the panel or introduce a second navigation shell. Per-site detail placement still requires auditing the actual site settings/detail view before choosing its exact insertion point.

### Linux (confirmed)
- The Sites pane uses a `gtk::Stack` with list/detail `PaneView` instances, `adw::PreferencesGroup`, `adw::ActionRow`, shared `info_row`, `group`, `status_dot`, and banner error reporting.
- Site detail is rendered by `linux/src/panes/sites.rs`; Services is a separate pane in `linux/src/panes/services.rs`.
- Integration rule: put site-specific connection metadata and launch action in the existing site detail stack, using shared Adwaita row/group helpers. Do not add Adminer to the database server catalog or create a separate site-details window. A global action must be reachable independently of site selection, using the existing main-pane/navigation pattern.

### Windows (confirmed)
- The tray panel is a WinUI XAML `ScrollViewer` / `StackPanel` with existing resource styles, status rows, warning `InfoBar`, and view-model bindings in `windows/Flock.Tray/PanelWindow.xaml`.
- `PanelWindow.xaml.cs` owns panel sizing, tray placement, theme updates, hide/show and view-model lifetime. Normal windows inherit Mica/title-bar behavior from `FlockWindow`.
- Integration rule: use existing XAML resource styles, view-model command/binding conventions and window/browser launch abstractions. Do not create a parallel visual framework or put a large Adminer UI inside the tray flyout. Exact site-detail insertion point and shared client DTO conventions must be checked in the Windows feature/view-model tree before implementation.

### Cross-platform consistency
The feature has the same semantics on all platforms, but native controls remain native. Global launch stays independent of a selected site. Per-site launch stays within that site's existing detail surface. The browser opens externally; credentials are never rendered into a URL or embedded web view. The wireframes must be adapted to the platform's current components rather than copied literally.
