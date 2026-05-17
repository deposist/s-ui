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
`s-ui admin -show`.
