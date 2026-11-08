# 05 — API and Data Contracts

All routes below are proposed. Final route registration must follow the existing API versioning and authentication conventions in `internal/api/api.go`.

## 1. Safe metadata DTO

```json
{
  "site_id": "site_opaque_id",
  "connections": [
    {
      "id": "conn_opaque_id",
      "label": "Primary MySQL",
      "engine": "mysql",
      "database": "storefront",
      "host": "127.0.0.1",
      "port": 3306,
      "username": "storefront_user",
      "source": "dotenv",
      "credential_configured": true,
      "status": "not_checked",
      "last_checked_at": null
    }
  ],
  "selected_connection_id": "conn_opaque_id",
  "discovery": {
    "status": "detected",
    "warnings": []
  }
}
```

Forbidden fields: password, raw DATABASE_URL, DSN, secret reference value, query parameters containing credentials, raw parser input.

## 2. Endpoints

### GET /v1/sites/{site_id}/database
Returns sanitized metadata and candidate connections belonging only to that site.

### POST /v1/database/connections/test
Request:
```json
{ "site_id": "site_opaque_id", "connection_id": "conn_opaque_id" }
```
Response:
```json
{ "status": "connected", "engine": "mysql", "latency_ms": 18, "checked_at": "RFC3339" }
```
Errors use stable codes such as `connection_refused`, `authentication_failed`, `timeout`, `unsupported_engine`, `approval_required`. Never return raw driver errors.

### POST /v1/database-admin/global/launch
Request:
```json
{ "tool": "adminer" }
```
Response:
```json
{ "launch_url": "http://127.0.0.1:PORT/_flock/launch/TICKET", "expires_in_seconds": 60, "tool": "adminer", "intent": "global" }
```
The URL contains only an opaque one-time ticket. It contains no site, DB name, username or password.

### POST /v1/sites/{site_id}/database-admin/launch
Request:
```json
{ "connection_id": "conn_opaque_id", "tool": "adminer" }
```
Response uses same shape, with `intent: "site"`. The server binds the site and connection to the ticket/session internally; do not expose the password or DSN.

### POST /v1/database-admin/stop
Request:
```json
{ "scope": "all" }
```
Response:
```json
{ "stopped": true, "sessions_revoked": 2 }
```

### Browser-only POST /_flock/exchange
This endpoint is on the dedicated loopback listener, not the general daemon API. It accepts the opaque ticket in a bounded POST body, atomically consumes it and establishes a cookie session. It rejects non-loopback peers, unexpected Host/Origin, expired tickets and replay.

## 3. Error envelope

```json
{
  "error": {
    "code": "database_connection_failed",
    "message": "Flock could not connect to this database.",
    "retryable": true,
    "details": { "reason": "authentication_failed" }
  }
}
```

Never include raw error strings, passwords, connection URLs, SQL statements or query result values in error details.

## 4. Identifier and scope rules

- Site and connection IDs are opaque, unguessable identifiers.
- Connection lookup must enforce `connection.site_id == requested_site_id`.
- A connection ID from Site A must fail when supplied with Site B.
- Global launch has no site scope and cannot inherit last-selected site state.
- Tickets are random, single-use, expire after at most 60 seconds and are stored only as a cryptographic hash.
- Ticket consumption must be atomic under concurrent requests.

## 5. Versioning

- Use the existing `/v1` API prefix.
- DTOs must be additive where possible.
- Native clients should tolerate unknown optional fields.
- Add contract tests proving secret fields never serialize.
