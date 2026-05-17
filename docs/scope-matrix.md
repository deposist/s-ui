# API Token Scope Matrix

Status: `1.5.1-beta`.

S-UI API tokens support four scopes: `admin`, `read`, `write`, and `observability`.
An empty scope is normalized to `admin` when a token is created. Browser cookie
sessions are treated as `admin` in the current single-admin model.

Use `Authorization: Bearer <token>` for `/apiv2/*`. The legacy `Token` header is
accepted during the sunset window only and returns `Deprecation` and `Sunset`
headers. Do not put API tokens into URLs.

## Scope Rules

| Endpoint or channel | Cookie session | `admin` | `read` | `write` | `observability` | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `GET /api/security/audit` | allowed | allowed | denied | denied | denied | Cursor pagination plus `event`, `severity`, `since`, and `until` filters. Denials write `audit_scope_denied`. |
| `GET /apiv2/security/audit` | n/a | allowed | denied | denied | denied | Bearer/API-token flow for audit reads. |
| `GET /api/observability/history` | allowed | allowed | denied | denied | allowed | Validates bucket, metric, and since query values. |
| `GET /api/observability/core-history` | allowed | allowed | denied | denied | allowed | Same scope policy as observability history. |
| `POST /api/telegram/test` | allowed | allowed | denied | denied | denied | Telegram remains off by default; proxy/token fields stay secret. |
| `POST /api/telegram/backup` | allowed | allowed | denied | denied | denied | Encrypted backup key is returned only in the response body, not in audit details. |
| `POST /apiv2/telegram/backup` | n/a | allowed | denied | denied | denied | Bearer/API-token flow for Telegram backup. |
| `GET /api/getdb` and `GET /apiv2/getdb` | allowed | allowed | denied | denied | denied | Database exports are audited. |
| `POST /api/importdb` and `POST /apiv2/importdb` | allowed | allowed | denied | denied | denied | Database imports are capped, integrity-checked, and audited. |
| `POST /api/rotateSubSecret` and `POST /apiv2/rotateSubSecret` | allowed | allowed | denied | allowed | denied | Rotates per-client subscription secrets and audits the action without logging the secret. |
| `/api/realtime/ws-token` + `/api/realtime/ws` | allowed | allowed | connected, filtered | connected, filtered | connected, filtered | Current browser flow is session-based. If a scoped context is present, `security_event` is delivered only to `admin`; other realtime topics follow the existing topic policy. |

For endpoints not listed above, `/api/*` still requires a browser session and
CSRF protection on mutating requests. `/apiv2/*` still requires a valid API token,
but not every legacy action has a dedicated per-action scope gate yet; use
`admin` unless the endpoint is explicitly listed in this matrix.

## Security Invariants

- Secret values are not returned by list/get endpoints; only marker or prefix
  fields are exposed where needed.
- Secret values must not be written to logs, audit details, config change
  history, or Telegram captions.
- API tokens must be sent in headers, not query strings.
- Security-relevant denials and state changes are audited without including raw
  tokens, subscription secrets, backup keys, proxy credentials, or Telegram bot
  tokens.
