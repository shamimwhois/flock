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
