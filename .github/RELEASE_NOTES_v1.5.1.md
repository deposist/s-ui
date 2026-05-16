# S-UI v1.5.1 - remediation hardening and UI completion

This release closes the remaining security, privacy, realtime, Telegram,
observability, frontend, and test gaps from the `1.5.0` remediation cycle.
The embedded `sing-box` runtime behavior is unchanged.

## Upgrade checklist

Back up the database before upgrading production:

```sh
sudo systemctl stop s-ui
sudo install -d -m 0700 /root/s-ui-backups
sudo cp -a /usr/local/s-ui/s-ui.db /root/s-ui-backups/s-ui.db.$(date +%Y%m%d%H%M%S)
sudo cp -a /usr/local/s-ui/s-ui.db-wal /root/s-ui-backups/ 2>/dev/null || true
sudo cp -a /usr/local/s-ui/s-ui.db-shm /root/s-ui-backups/ 2>/dev/null || true
sudo systemctl start s-ui
```

Do not publish database backups, subscription URLs, private keys,
certificates, admin credentials, API tokens, Telegram bot tokens, proxy
credentials, or Telegram backup keys in issues, pull requests, CI logs, or
support chats.

## Highlights

- Telegram notifications are asynchronous with a bounded queue, drop-oldest
  overflow handling, retry/backoff, and audit events for failure/overflow.
- Telegram secrets and proxy credentials are encrypted at rest, masked from
  settings responses, redacted from audit/change history, and never included
  in Telegram backup captions.
- Realtime WebSocket handshakes now enforce Origin checks, per-IP rate limits,
  single-use tokens, heartbeat/idle close, and close-all on session rotation.
- Audit history is admin-scoped for API tokens, rate limited, cursor paginated,
  and filterable by validated `event` and `severity`.
- Client IP history is hashed and masked by default, with retention GC and
  monitor-only default behavior. Enforce mode rejects only new over-limit
  connections.
- Subscription settings, path validation, header sanitization, configurable
  rate limits, and per-client secret rotation are complete. With
  `subSecretRequired=true`, legacy name URLs return 404.
- Telegram proxy egress, normalized Telegram error classes, CPU hysteresis
  alerts, scheduled reports, and encrypted Telegram DB backup export are
  implemented and remain opt-in.
- Observability history uses bounded buckets sampled by cron and validated API
  parameters. Logs and version endpoints are bounded and fail-soft.
- Frontend now includes websocket fallback state, secret-aware settings fields,
  masked IP history modal, Telegram settings, and Audit views.

## Compatibility

- Existing subscription name URLs still work while `subSecretRequired=false`.
- Legacy `/apiv2/*` `Token` header still works for now, but responses include
  `Deprecation` and `Sunset` headers.
- Legacy `Token` header sunset date: **Sat, 15 Aug 2026 00:00:00 GMT**.
  Move integrations to `Authorization: Bearer <token>` before that date.
- All new features remain off by default except realtime websocket support
  with frontend polling fallback and monitor-only IP tracking.

## Verification

Passed in this workspace:

- `go vet ./...`
- `go test ./...`
- `npm run test:unit`
- `npm run build`
- `npm run lint`

`go test -race ./...` could not run in this Windows workspace because the Go
race detector requires CGO and no C compiler is available:

```text
cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%
```

Install GCC, for example via MSYS2/UCRT64, then rerun:

```powershell
$env:CGO_ENABLED='1'
go test -race ./...
```

## Rollback

If rollback is required:

1. Stop the service.
2. Restore the backed-up `s-ui.db` and any matching `-wal`/`-shm` sidecars.
3. Restore the previous full release archive or container image.
4. Start the service.

Keep the `SUI_SECRETBOX_KEY` value stable across upgrade and rollback. Losing
that key can make encrypted settings unreadable.
