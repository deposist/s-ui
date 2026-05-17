# Changelog

The full changelog is split per language. Pick the file that matches your
preferred language:

- English — [`CHANGELOG-EN.md`](CHANGELOG-EN.md)
- Русский — [`CHANGELOG-RU.md`](CHANGELOG-RU.md)
- 简体中文 — [`CHANGELOG-ZH.md`](CHANGELOG-ZH.md)

All three files cover the same set of releases and are kept in sync.

Current `1.5.1-beta` remediation notes include moving fresh-install admin
password disclosure out of logs and into `<dataDir>/initial-admin.txt` with
owner-only permissions, plus hiding stored password hashes from
`s-ui admin -show` and clearing stale CSRF tokens after logout/session
rotation. The installer now verifies release tarballs with published SHA256
checksums before extraction. Pull requests now run backend and frontend CI
checks automatically.
Admin web sessions now use a SQLite-backed server-side store, so the browser
cookie contains only a signed session ID while session data is stored in the
local `sessions` table.
Docker builds now document the `cronet-go` source pin used by release builds
and the dated fallback to upstream's latest prebuilt `libcronet` asset.
The Docker image default timezone now matches the panel default
`Europe/Moscow`.
The manual release workflow now defaults to tag `v1.5.1-beta`.
The container entrypoint no longer runs a duplicate automatic migration before
startup; use `SUI_MIGRATE_ONLY=1` for a manual migration-only run.
The migration runner now executes the WAL checkpoint only after a successful
transaction commit, avoiding `database table is locked` failures during
upgrades from `1.4.x`.
