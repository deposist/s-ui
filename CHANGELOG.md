# Changelog

## Summary
Implemented security, reliability, frontend tooling, and performance improvements across S-UI. The main validation path now passes with backend tests/vet, frontend install/build/lint, and npm audit.

## Security
- Replaced plaintext password storage with bcrypt hashes.
- Added lazy migration for existing plaintext passwords on successful login.
- Removed fixed default `admin/admin`; first-run admin password is now randomly generated and logged.
- Added login rate limiting.
- Hardened session cookies with `HttpOnly`, `SameSite=Lax`, and HTTPS-aware `Secure`.
- Stopped trusting `X-Forwarded-For` unless `SUI_TRUSTED_PROXIES` is configured.
- Replaced unsafe SQL string concatenation with parameterized queries.
- Removed default TLS verification bypass for external subscription fetches.
- Added external subscription URL validation:
  - only `http`/`https`
  - blocks localhost/private/link-local targets by default
  - can be relaxed with `SUI_ALLOW_PRIVATE_SUB_URLS=true`
  - limits response size

## Reliability / Data Integrity
- Fixed backup export to include `services` and API `tokens`.
- Fixed database path construction using platform-safe path joins.
- Added database indexes for common stats/change/client lookups.
- Started checking transaction commit errors.
- Changed config/runtime update flow so sing-box runtime changes happen after successful DB commit.
- Added synchronization around API tokens, online stats, core running state, and last update tracking.
- Added HTTP server read/write/header timeouts and TLS minimum version.

## Frontend / Tooling
- Fixed `npm ci` by syncing `package-lock.json`.
- Migrated ESLint to flat config.
- Updated lint script to run checks without auto-fixing.
- Fixed npm audit issues; audit now reports 0 vulnerabilities.
- Moved Axios interceptors onto the exported Axios instance.
- Replaced deprecated Axios `CancelToken` usage with `AbortController`.
- Removed unsafe `v-html` usage for logs, rule import errors, and displayed IP lists.
- Fixed frontend state bugs:
  - `enableTraffic=false` now updates correctly
  - `loadClients` safely handles empty results
  - filtered status request list is now actually used
- Enabled frontend code splitting instead of forcing a single monolithic bundle.

## Tests
- Added regression tests for:
  - password hashing and plaintext migration behavior
  - external URL validation
  - default port omission in generated subscription URI
  - backup inclusion of `services` and `tokens`
- Added `frontend/go.mod` so root Go commands no longer traverse `frontend/node_modules`.

## Verification
Passed:
- `go test ./...`
- `go vet ./...`
- `go test -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale" ./...`
- `npm ci`
- `npm run build`
- `npm run lint`
- `npm audit --audit-level=high`

Not run successfully:
- `go test -race ./...` requires CGO and a C compiler. In the current environment, `gcc` is not available in `PATH`.
