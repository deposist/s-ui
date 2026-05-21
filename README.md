## S-UI

<p align="center">
  <img width="492" height="450" alt="s-ui-rus-inst logo" src="https://github.com/user-attachments/assets/cfc9da97-f8ea-4c68-961c-2bf164932272" />
</p>
<p align="center">
  <a href="https://github.com/deposist/s-ui-rus-inst/releases/latest">
    <img src="https://img.shields.io/github/v/release/deposist/s-ui-rus-inst?style=for-the-badge&label=release" alt="Release">
  </a>
  <a href="https://github.com/deposist/s-ui-rus-inst/releases">
  <a href="https://github.com/deposist/s-ui-rus-inst/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/deposist/s-ui-rus-inst?style=for-the-badge" alt="License">
  </a>
  <a href="https://github.com/deposist/s-ui-rus-inst/stargazers">
    <img src="https://img.shields.io/github/stars/deposist/s-ui-rus-inst?style=for-the-badge" alt="Stars">
  </a>
</p>

## English

Advanced Web panel built on `SagerNet/Sing-Box`.

**Note:** the original `alireza0/s-ui` project was blocked and removed by GitHub. This repository is a complete backup based on the last original version, `v1.4.1`, with security and reliability hardening applied on top (current build: `v1.5.3-beta`).

**This fork keeps the original project structure and updates the user-facing documentation and installation links for this repository. You can use the scripts from this repository directly, or fork and build the project yourself.**

> **Disclaimer:** this project is intended only for personal learning and knowledge sharing. Do not use it for illegal purposes.

## Releases

The full per-release notes live in the language-specific changelog files:

- English: [`CHANGELOG-EN.md`](CHANGELOG-EN.md)
- Русский: [`CHANGELOG-RU.md`](CHANGELOG-RU.md)
- 简体中文: [`CHANGELOG-ZH.md`](CHANGELOG-ZH.md)

Short summary of recent versions:

| Version | One-line summary |
| --- | --- |
| `1.5.3-beta` | Code review remediation aggregate (P0/P1/P2/P3 + P4/P5 architecture and logging cleanup) plus upstream parity bug fix for [alireza0/s-ui#1114](https://github.com/alireza0/s-ui/issues/1114): TUIC subscription/share links and Clash export now include `udp_relay_mode` (default `quic` when absent). Logging fully migrated to `slog`, deprecated `op/go-logging` dependency removed, restart manager unified, listen-address fallback audited, and globals reduced via initial DI slice. |
| `1.5.2-beta-hotfix2` | Drops the legacy `idx_client_ips_client_ip` unique index on `(client_name, ip)`. The 3x-ui pre-import auto-backup no longer crashes with `UNIQUE constraint failed: client_ips.client_name, client_ips.ip` when a client owns multiple `client_ips` rows with empty legacy `ip`. `to1_5` is idempotent and runs again on already-upgraded panels; `ensureIndexes` drops the obsolete index at every `InitDB` so the temporary backup DB stops carrying it too. |
| `1.5.2-beta-hotfix` | Backup chunking and SPA upgrade safety: `database/bulk.go` keeps every backup INSERT under SQLite's variable budget, fixing `too many SQL variables` on installs with large `stats`/`client_ips`/`clients`/`changes` tables and unblocking the 3x-ui pre-import auto-backup. `web/web.go` returns 404 for missing `/<base>/assets/*` instead of HTML, `index.html` is no-cache, and `vite:preloadError` triggers one guarded reload so the Clients tab stops breaking after upgrades. |
| `1.5.2-beta` | 3x-ui migration suite: `s-ui import-xui` CLI, `POST /api/import-xui` HTTP API, "Migrate from 3x-ui" section in Backup & Restore, full wizard at `/migrate-xui` (plan/apply/rollback, WS progress, JSON/Markdown report), remote SSH and `xuihttp` sources, `xuiSyncJob` cron with encrypted `xui_sync_profiles`, `/migrate-xui/schedule` UI, best-effort historical traffic and Xray routing rules import, new `xui_remote` token scope. |
| `1.5.1-beta` | Remediation hardening: async Telegram queue, redacted payloads, hardened realtime WS handshake, scoped audit endpoint, hashed/retained client IPs, Telegram proxy egress with normalized error classes, bucketed observability, frontend completion. |
| `1.5.0` | Security foundation and realtime platform: secretbox for sensitive settings, `audit_events` + `/api/security/audit`, CSRF for browser API, hashed/scoped API tokens, Bearer auth (legacy `Token` header deprecated), per-client subscription secrets, `/api/realtime/ws*`, IP monitor (monitor-only by default). |
| `1.4.3` | sing-box runtime update from `v1.13.4` to `v1.13.11`. No DB or UI changes. |
| `1.4.2-beta` | Security and reliability hardening: bcrypt password storage with lazy migration, login rate limiter, hardened cookies, SSRF-resistant subscription fetcher, parameterized SQL, race-free runtime, automatic legacy backup adaptation, bilingual installer. Module renamed to `github.com/deposist/s-ui-rus-inst`. |

For full changes, breaking notes, upgrade guide, and rollback steps, open the changelog in your preferred language.

## Key differences vs `alireza0/s-ui`

<details>
  <summary>Show details</summary>

This fork is binary-compatible with `alireza0/s-ui` — drop the new
binary on top of an existing 1.x install, the panel migrates the DB
automatically on first start. The intent is to harden security and
reliability without changing the protocol surface.

- **Auth and session security.** bcrypt with lazy migration, randomly generated first-run password (logged once), login rate limiter, `HttpOnly` + `SameSite=Lax` + HTTPS-aware `Secure` cookies. Sensitive settings (Telegram bot token, proxy credentials, install salt) are encrypted at rest via secretbox; API tokens are stored as salted SHA-256 hashes. CSRF protection is enforced on browser `/api/*` mutating requests.
- **API token scopes.** `admin`, `read`, `write`, and `observability` scopes are documented in [`docs/scope-matrix.md`](docs/scope-matrix.md), including audit, database, Telegram, subscription-secret rotation, observability, and realtime security-event behavior.
- **`X-Forwarded-For` handling.** Header is ignored unless `SUI_TRUSTED_PROXIES` is configured; the chain is walked right-to-left so spoofed XFF cannot reach IP-based logic.
- **External subscription fetcher.** URL allow-list, blocks private/loopback targets by default (opt-in via `SUI_ALLOW_PRIVATE_SUB_URLS=true`), 4 MiB response cap, DNS-rebinding-safe dial-time IP re-validation. `Authorization: Bearer <token>` is the primary API token transport on `/apiv2/*`; the legacy `Token` header still works with `Deprecation` and `Sunset` headers until `Sat, 15 Aug 2026 00:00:00 GMT`.
- **Realtime WebSocket.** `/api/realtime/ws-token` + `/api/realtime/ws` enforce Origin allow-listing, per-IP handshake rate limits, single-use tokens, ping/pong heartbeat, idle close, and close-all on session rotation. Frontend has a polling fallback for degraded states.
- **Per-client subscription secrets.** `/sub/<secret>`, `/sub/json/<secret>`, `/sub/clash/<secret>`, `/json/<secret>`, `/clash/<secret>` are supported; legacy `/sub/<name>` keeps working until `subSecretRequired=true`. Subscription endpoints sanitize response headers and apply a configurable per-IP rate limit.
- **Telegram notifications (off by default).** Async bounded queue with retry/backoff and audited overflow/failure events. Egress can use validated HTTP/HTTPS/SOCKS5 proxy settings. Telegram payloads, audit details, change history, and backup captions are redacted.
- **Audit and observability.** `audit_events` table with retention GC, scoped `GET /api/security/audit` endpoint with rate limiting and cursor pagination. Bounded observability buckets (`2s`, `30s`, `1m`, `5m`) sampled by cron. Bounded logs API and fail-soft 1h-cached `GET /api/version`.
- **IP monitor (monitor-only by default).** Salted hashes, opt-in raw display, retention GC, per-client `limitIp` and `ipLimitMode`. Enforce mode rejects only new over-limit connections and never closes active connections.
- **SQL safety.** Parameterized queries throughout `service/config.go` and `service/inbounds.go`; static allow-list of inbound types in the user-fetch SQL builder.
- **Backup import / upgrade.** `ImportDB` enforces a 64 MiB cap, SQLite magic check, temporary staging, read-only `PRAGMA integrity_check`, and audit events. WAL/SHM sidecars are cleaned, schema migrations + `AdaptToCurrentVersion` run automatically (rehashes legacy plaintext passwords, refreshes indexes, bumps `settings.version`); the previous DB is restored on any failure.
- **Listen-address resilience.** When the saved `webListen` / `subListen` IP no longer exists on the host, the panel logs a warning and binds on every interface instead of failing with `EADDRNOTAVAIL`.
- **Race-free runtime.** Core lifecycle, online stats, last-update bookkeeping, v2 token store, realtime hub all pass `go test -race ./...` (requires CGO).
- **HTTP server hardening.** `Read/Write/Header/Idle` timeouts and `tls.MinVersion = 1.2` on both the panel and the subscription endpoint. Security-headers middleware (CSP, HSTS when TLS, no-store on subscription responses).
- **WARP registration.** Talks to the current Cloudflare WARP API (`v0a4005`) with proper first-party headers, falls back to `v0a2158`, retries transient TLS handshake failures.
- **Frontend hygiene.** `v-html` removed from logs, rule import errors, IP lists, and the gauge tile. Axios on an exported instance, `AbortController` instead of deprecated `CancelToken`, dedupe limited to idempotent reads, Vite code splitting on. Realtime WS store with reconnect/degraded states. Secret-aware settings fields with `••• stored •••` placeholder. IP history modal with raw-IP masking by default. Telegram settings and Audit views.
- **Localization & defaults.** Multilingual `install.sh` and `s-ui` management menu (English / Russian / Chinese), language switchable at runtime. Default `timeLocation` is `Europe/Moscow`. Default frontend locale is English (existing browsers keep their `localStorage` choice).

</details>

## Overview

| Feature | Support |
| -------------------------------------- | :----------------: |
| Multiple protocols | :heavy_check_mark: |
| Multiple languages | :heavy_check_mark: |
| Multiple clients/inbounds | :heavy_check_mark: |
| Advanced traffic routing interface | :heavy_check_mark: |
| Client, traffic, and system status | :heavy_check_mark: |
| Subscription links (link/json/clash + info) | :heavy_check_mark: |
| Dark/light theme | :heavy_check_mark: |
| API | :heavy_check_mark: |

## Supported Platforms

| Platform | Architecture | Status |
|----------|--------------|---------|
| Linux | amd64, arm64, armv7, armv6, armv5, 386, s390x | Supported |
| Windows | amd64, 386, arm64 | Supported |
| macOS | amd64, arm64 | Experimental support |

## Default Installation Information

- Panel port: 2095
- Panel path: /app/
- Subscription port: 2096
- Subscription path: /sub/
- Subscription per-IP rate-limit changes (`subRateLimitPerIP`) take effect within 1 minute after saving.
- Username: admin
- Password (fresh install only): a random 24-character string is generated on first start and written to the application log. Look for the line `created initial admin user. username=admin password=...` in `journalctl -u s-ui` (Linux) or in the panel log on first run. After that, change it from the panel.

## Install or Upgrade to the Latest Stable Version

### Linux/macOS

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/install.sh)
```

### Windows

1. Download the latest Windows version from [GitHub Releases](https://github.com/deposist/s-ui-rus-inst/releases/latest).
2. Extract the ZIP file.
3. Run `install-windows.bat` as Administrator.
4. Follow the installation wizard.

## Install v1.5.3-beta (current beta)

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/install.sh) v1.5.3-beta
```

Or from a local clone:

```sh
git clone -b beta https://github.com/deposist/s-ui-rus-inst.git
cd s-ui-rus-inst
sudo bash install.sh v1.5.3-beta
```

Or from a local clone:

```sh
git clone -b beta .git
cd s-ui-rus-inst
sudo bash install.sh v1.5.1-beta
```

The installer is fully compatible with existing installations: settings,
inbounds, outbounds, clients, TLS, services and tokens are kept; the DB
schema is migrated automatically on first start; plaintext admin
passwords are upgraded to bcrypt on the next successful login. Full
upgrade procedure and rollback notes — in the per-language changelog
([EN](CHANGELOG-EN.md) · [RU](CHANGELOG-RU.md) · [中文](CHANGELOG-ZH.md)).

## Install an Older Version

Append the version tag with `v` to the installation command. For example, version `v1.0.0`:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/install.sh) v1.0.0
```

## Manual Installation

### Linux/macOS

1. Download the latest S-UI version for your system and architecture from GitHub: [https://github.com/deposist/s-ui-rus-inst/releases/latest](https://github.com/deposist/s-ui-rus-inst/releases/latest)
2. **Optional:** download the latest `s-ui.sh`: [https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/s-ui.sh](https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/s-ui.sh)
3. **Optional:** copy `s-ui.sh` to `/usr/bin/` and run `chmod +x /usr/bin/s-ui`.
4. Extract the s-ui tar.gz archive to your chosen directory and enter the extracted folder.
5. Copy the `*.service` files to `/etc/systemd/system/`, then run `systemctl daemon-reload`.
6. Run `systemctl enable s-ui --now` to enable autostart and start the S-UI service.
7. Run `systemctl enable sing-box --now` to start the sing-box service.

### Windows

1. Download the latest Windows version from GitHub: [https://github.com/deposist/s-ui-rus-inst/releases/latest](https://github.com/deposist/s-ui-rus-inst/releases/latest)
2. Download the appropriate Windows package, for example `s-ui-windows-amd64.zip`.
3. Extract the ZIP file to your chosen directory.
4. Run `install-windows.bat` as Administrator.
5. Follow the installation wizard.
6. Open the panel: http://localhost:2095/app

## Uninstall S-UI

```sh
sudo -i

systemctl disable s-ui  --now

rm -f /etc/systemd/system/sing-box.service
systemctl daemon-reload

rm -fr /usr/local/s-ui
rm /usr/bin/s-ui
```

## Docker Installation

<details>
   <summary>Show details</summary>

### Usage

**Step 1:** install Docker

```shell
curl -fsSL https://get.docker.com | sh
```

**Step 2:** install S-UI

> Docker Compose option

```shell
services:
  s-ui:
    image: ghcr.io/deposist/s-ui-rus-inst
    container_name: s-ui
    hostname: "s-ui"
    network_mode: host
    volumes:
      - "./db:/app/db"
      - "./cert:/app/cert"
    tty: true
    restart: unless-stopped
    entrypoint: "./entrypoint.sh"
```

`docker compose up -d`

> Direct Docker run

```shell
mkdir s-ui && cd s-ui

docker run -itd \
    --network host \
    -v $PWD/db/:/app/db/ \
    -v $PWD/cert/:/root/cert/ \
    --name s-ui \
    --restart=unless-stopped \
    ghcr.io/deposist/s-ui-rus-inst
```

> Build the image yourself

```shell
git clone https://github.com/deposist/s-ui-rus-inst
docker build -t s-ui .
```

</details>

## Manual Run for Development and Contributions

<details>
   <summary>Show details</summary>

### Build and Run the Full Project

```shell
./runSUI.sh
```

### Clone the Repository

```shell
# Clone the repository
git clone https://github.com/deposist/s-ui-rus-inst
```

### Frontend

The frontend code is in the [frontend](frontend) directory.

### Backend

> Build the frontend at least once before building the backend.

Build the backend:

```shell
# Remove old frontend build files
rm -fr web/html/*
# Copy new frontend build files
cp -R frontend/dist/ web/html/
# Build
go build -o sui main.go
```

Run the backend from the repository root:

```shell
./sui
```

</details>

## Languages

- English
- Persian
- Vietnamese
- Simplified Chinese
- Traditional Chinese
- Russian

## Features

- Supported protocols:
  - General protocols: Mixed, SOCKS, HTTP, HTTPS, Direct, Redirect, TProxy
  - V2Ray-based protocols: VLESS, VMess, Trojan, Shadowsocks
  - Other protocols: ShadowTLS, Hysteria, Hysteria2, Naive, TUIC
- XTLS protocol support.
- Advanced traffic routing interface with PROXY Protocol, External, transparent proxy, SSL certificates, and port configuration support.
- Advanced inbound and outbound configuration interface.
- Client traffic limit and expiration support.
- Online clients, inbound/outbound traffic statistics, and system status monitoring.
- Subscription service supports external links and subscriptions.
- Web panel and subscription service support secure HTTPS access (you must provide your own domain and SSL certificate).
- Dark/light theme.

## Environment Variables

<details>
  <summary>Show details</summary>

### Usage

| Variable | Type | Default |
| -------------- | :--------------------------------------------: | :------------ |
| SUI_LOG_LEVEL | `"debug"` \| `"info"` \| `"warn"` \| `"error"` | `"info"` |
| SUI_DEBUG | `boolean` | `false` |
| SUI_BIN_FOLDER | `string` | `"bin"` |
| SUI_DB_FOLDER | `string` | `"db"` |
| SINGBOX_API | `string` | - |
| SUI_TRUSTED_PROXIES | comma-separated CIDRs / IPs | - (XFF ignored) |
| SUI_ALLOW_PRIVATE_SUB_URLS | `boolean` | `false` |
| SUI_SECRETBOX_KEY | `string` | - (falls back to `settings.secret`) |

</details>

## SSL Certificates

<details>
  <summary>Show details</summary>

### Certbot

```bash
snap install core; snap refresh core
snap install --classic certbot
ln -s /snap/bin/certbot /usr/bin/certbot

certbot certonly --standalone --register-unsafely-without-email --non-interactive --agree-tos -d <your domain>
```

</details>

#### Credits to the original author: alireza0

---

## Русский

Продвинутая Web-панель, построенная на базе `SagerNet/Sing-Box`.

**Примечание:** оригинальный проект `alireza0/s-ui` был заблокирован и удалён GitHub. Этот репозиторий — полная резервная копия последней оригинальной версии `v1.4.1` с применённым набором исправлений по безопасности и надёжности (текущая сборка: `v1.5.3-beta`).

**Этот fork сохраняет структуру оригинального проекта и обновляет пользовательскую документацию и ссылки установки для этого репозитория. Вы можете напрямую использовать скрипты из этого репозитория или сделать fork и собрать проект самостоятельно.**

> **Отказ от ответственности:** этот проект предназначен только для личного обучения и обмена опытом. Не используйте его в незаконных целях.

## Релизы

Полные release notes лежат в отдельных файлах changelog по языкам:

- English: [`CHANGELOG-EN.md`](CHANGELOG-EN.md)
- Русский: [`CHANGELOG-RU.md`](CHANGELOG-RU.md)
- 简体中文: [`CHANGELOG-ZH.md`](CHANGELOG-ZH.md)

Краткая сводка по последним версиям:

| Версия | Однострочное описание |
| --- | --- |
| `1.5.3-beta` | Агрегат исправлений code-review (P0/P1/P2/P3 + P4/P5 архитектурный долг и logging cleanup) и upstream-парити-фикс по [alireza0/s-ui#1114](https://github.com/alireza0/s-ui/issues/1114): TUIC subscription/share links и Clash export теперь включают `udp_relay_mode` (по умолчанию `quic`, если режим не задан). Логирование полностью переведено на `slog`, зависимость `op/go-logging` удалена, restart-manager унифицирован, поведение listen-address fallback задокументировано, globals частично вынесены в initial DI slice. |
| `1.5.2-beta-hotfix2` | Снятие legacy unique-индекса `idx_client_ips_client_ip` на `(client_name, ip)`. Автобэкап перед миграцией с 3x-ui больше не падает с `UNIQUE constraint failed: client_ips.client_name, client_ips.ip` при наличии нескольких строк `client_ips` с пустым legacy `ip`. `to1_5` идемпотентна и повторно прогоняется на уже обновлённых панелях; `ensureIndexes` дропает устаревший индекс на каждом `InitDB`, чтобы временная backup-БД его тоже не получала. |
| `1.5.2-beta-hotfix` | Чанки в бэкапе и безопасность SPA при апгрейде: `database/bulk.go` держит каждый INSERT бэкапа в пределах бюджета переменных SQLite, что чинит `too many SQL variables` на инсталлах с большими `stats`/`client_ips`/`clients`/`changes` и разблокирует автобэкап перед миграцией с 3x-ui. `web/web.go` возвращает 404 для отсутствующих `/<base>/assets/*` вместо HTML, `index.html` отдаётся с no-cache, а `vite:preloadError` делает одну защищённую перезагрузку — вкладка Clients больше не ломается после апгрейда. |
| `1.5.2-beta` | Пакет миграции из 3x-ui: CLI `s-ui import-xui`, HTTP-API `POST /api/import-xui`, секция «Migrate from 3x-ui» в Backup & Restore, полный мастер на `/migrate-xui` (plan/apply/rollback, прогресс по WS, отчёт JSON/Markdown), удалённые источники SSH и `xuihttp`, cron-job `xuiSyncJob` с зашифрованными `xui_sync_profiles`, страница `/migrate-xui/schedule`, best-effort импорт исторического трафика и Xray routing-правил, новый scope токена `xui_remote`. |
| `1.5.1-beta` | Закрытие техдолга и UI: асинхронная очередь Telegram, redaction payload'ов, hardened realtime WS handshake, scoped audit endpoint, hashed/retained client IPs, Telegram через прокси с нормализованными errorClass, бакетированная observability, готовый фронт. |
| `1.5.0` | Фундамент безопасности и realtime: secretbox для секретов, `audit_events` + `/api/security/audit`, CSRF для browser API, hashed/scoped API tokens, Bearer auth (legacy `Token` deprecated), per-client subscription secrets, `/api/realtime/ws*`, IP monitor (monitor-only по умолчанию). |
| `1.4.3` | Обновление sing-box runtime с `v1.13.4` до `v1.13.11`. БД и UI без изменений. |
| `1.4.2-beta` | Хардеринг безопасности и надёжности: bcrypt с ленивой миграцией, login rate limiter, защищённые cookie, SSRF-защищённый загрузчик подписок, параметризованный SQL, race-free runtime, авто-адаптация легаси-бэкапов, двуязычный установщик. Модуль переименован в `github.com/deposist/s-ui-rus-inst`. |

Полный список изменений, breaking-заметки, гайд по обновлению и инструкции по откату — в выбранном вами changelog.

## Ключевые отличия от `alireza0/s-ui`

<details>
  <summary>Показать подробности</summary>

Этот форк бинарно совместим с `alireza0/s-ui` — новый бинарник можно
ставить поверх работающей установки 1.x, схема БД автоматически
обновится при первом старте. Цель форка — усилить безопасность и
надёжность, не меняя протокол.

- **Авторизация и сессия.** bcrypt с ленивой миграцией, случайный пароль администратора при первой установке (выводится в журнал один раз), лимит на неуспешные логины, cookie сессии — `HttpOnly` + `SameSite=Lax` + `Secure` при HTTPS. Чувствительные настройки (Telegram bot token, креденшелы прокси, install salt) шифруются at-rest через secretbox; API-токены хранятся как salted SHA-256. CSRF-защита на browser `/api/*`-mutating-запросах.
- **Scopes API-токенов.** `admin`, `read`, `write` и `observability` описаны в [`docs/scope-matrix.md`](docs/scope-matrix.md), включая audit, database, Telegram, rotation subscription-secret, observability и realtime security-event.
- **`X-Forwarded-For`.** Заголовок игнорируется без `SUI_TRUSTED_PROXIES`; цепочка обходится справа налево, поддельный XFF не может обойти IP-логику.
- **Загрузчик внешних подписок.** Allow-list URL, блок приватных/loopback по умолчанию (opt-in `SUI_ALLOW_PRIVATE_SUB_URLS=true`), лимит ответа 4 МиБ, защита от DNS rebinding на dial. `Authorization: Bearer <token>` — основной способ передачи API-токена в `/apiv2/*`; legacy `Token`-header работает с `Deprecation` и `Sunset` до `Sat, 15 Aug 2026 00:00:00 GMT`.
- **Realtime WebSocket.** `/api/realtime/ws-token` + `/api/realtime/ws` с Origin allow-list, per-IP rate-limit handshake, одноразовыми токенами, ping/pong heartbeat, idle close и close-all при ротации сессии. На фронте есть polling-фолбэк для degraded-состояний.
- **Per-client subscription secrets.** Поддерживаются `/sub/<secret>`, `/sub/json/<secret>`, `/sub/clash/<secret>`, `/json/<secret>`, `/clash/<secret>`; legacy `/sub/<name>` работает пока `subSecretRequired=false`. Subscription-эндпоинты санитизируют response-заголовки и применяют конфигурируемый per-IP rate-limit.
- **Telegram-уведомления (off by default).** Асинхронная bounded-очередь с retry/backoff и audit-событиями overflow/failure. Egress может идти через валидированные HTTP/HTTPS/SOCKS5-прокси. Payload, audit-детали, changes и captions проходят redaction.
- **Audit и observability.** Таблица `audit_events` с retention GC, scoped эндпоинт `GET /api/security/audit` с rate-limit и cursor pagination. Bounded observability buckets (`2s`, `30s`, `1m`, `5m`), сэмплятся cron'ом. Bounded logs API и fail-soft 1h-cached `GET /api/version`.
- **IP monitor (monitor-only по умолчанию).** Соль+SHA-256 hashing, opt-in raw-display, retention GC, per-client `limitIp` и `ipLimitMode`. Enforce отбрасывает только новые сверхлимитные подключения и не разрывает активные.
- **Безопасность SQL.** Параметризованные запросы в `service/config.go` и `service/inbounds.go`; в выборке пользователей по inbound — статический whitelist допустимых типов.
- **Импорт бэкапа / обновление.** `ImportDB` имеет cap 64 МиБ, проверку SQLite magic, временную staging-копию, read-only `PRAGMA integrity_check` и audit-события. WAL/SHM сайдкары очищаются, schema-миграции и `AdaptToCurrentVersion` запускаются автоматически (перешивка plaintext-паролей в bcrypt, обновление индексов, поднятие `settings.version`); при ошибке БД восстанавливается из staging.
- **Листен-адрес, устойчивый к переезду.** Если в `webListen` / `subListen` сохранён IP, которого нет на текущем хосте, панель пишет warning и слушает на всех интерфейсах вместо краша `EADDRNOTAVAIL`.
- **Race-free runtime.** core lifecycle, online-stats, last-update, v2 token store и realtime hub проходят `go test -race ./...` (требует CGO).
- **HTTP server hardening.** Таймауты `Read/Write/Header/Idle` и `tls.MinVersion = 1.2` для панели и для эндпоинта подписки. Middleware security-headers (CSP, HSTS при TLS, no-store на subscription-ответах).
- **WARP-регистрация.** Поддержка актуального API Cloudflare (`v0a4005`) с заголовками первого клиента, фоллбэк на `v0a2158`, ретраи переходящих TLS-ошибок.
- **Чистота фронтенда.** `v-html` удалён из логов, ошибок импорта правил, IP-листов и gauge-плитки. Axios через экспортируемый instance, `AbortController` вместо устаревшего `CancelToken`, дедупликация только для идемпотентных запросов, code splitting Vite. Realtime WS-store с reconnect/degraded состояниями. Secret-aware-поля настроек с placeholder'ом `••• stored •••`. IP-history modal с маской raw-IP по умолчанию. Views Telegram-настроек и Audit.
- **Локализация и значения по умолчанию.** Многоязычные `install.sh` и меню `s-ui` (английский / русский / китайский), язык переключается на лету. Часовой пояс по умолчанию — `Europe/Moscow`. Локаль фронтенда по умолчанию — английский (существующие браузеры сохраняют выбор из `localStorage`).

</details>

## Краткий обзор

| Возможность | Поддержка |
| -------------------------------------- | :----------------: |
| Несколько протоколов | :heavy_check_mark: |
| Несколько языков | :heavy_check_mark: |
| Несколько клиентов/входящих подключений | :heavy_check_mark: |
| Продвинутый интерфейс маршрутизации трафика | :heavy_check_mark: |
| Клиенты, трафик и состояние системы | :heavy_check_mark: |
| Ссылки подписки (link/json/clash + info) | :heavy_check_mark: |
| Темная/светлая тема | :heavy_check_mark: |
| API | :heavy_check_mark: |

## Поддерживаемые платформы

| Платформа | Архитектура | Статус |
|----------|--------------|---------|
| Linux | amd64, arm64, armv7, armv6, armv5, 386, s390x | Поддерживается |
| Windows | amd64, 386, arm64 | Поддерживается |
| macOS | amd64, arm64 | Экспериментальная поддержка |

## Информация об установке по умолчанию

- Порт панели: 2095
- Путь панели: /app/
- Порт подписки: 2096
- Путь подписки: /sub/
- Изменения лимита подписок на IP (`subRateLimitPerIP`) применяются в течение 1 минуты после сохранения.
- Имя пользователя: admin
- Пароль (только для свежей установки): при первом запуске генерируется случайная строка из 24 символов, которая выводится в журнал приложения. Найдите строку `created initial admin user. username=admin password=...` в `journalctl -u s-ui` (Linux) или в журнале панели после первого запуска. После входа смените пароль в настройках.

## Установка или обновление до последней стабильной версии

### Linux/macOS

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/install.sh)
```

### Windows

1. Скачайте последнюю версию для Windows из [GitHub Releases](https://github.com/deposist/s-ui-rus-inst/releases/latest).
2. Распакуйте ZIP-файл.
3. Запустите `install-windows.bat` от имени администратора.
4. Следуйте инструкциям мастера установки.

## Установка v1.5.3-beta (текущая бета)

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/install.sh) v1.5.3-beta
```

Или из локального клона:

```sh
git clone -b beta https://github.com/deposist/s-ui-rus-inst.git
cd s-ui-rus-inst
sudo bash install.sh v1.5.3-beta
```

Либо из локального клона:

```sh
git clone -b beta https://github.com/deposist/s-ui-rus-inst.git
cd s-ui-rus-inst
sudo bash install.sh v1.5.1-beta
```

Установщик полностью совместим с уже работающими установками: настройки,
inbounds, outbounds, клиенты, TLS, services и токены сохраняются; схема
БД мигрируется автоматически при первом запуске; пароль администратора
в открытом виде заменяется на bcrypt-хеш при следующем успешном входе.
Полный гайд по обновлению и откату — в changelog'е на нужном языке
([EN](CHANGELOG-EN.md) · [RU](CHANGELOG-RU.md) · [中文](CHANGELOG-ZH.md)).

## Установка старой версии

Чтобы установить определённую старую версию, добавьте тег версии с `v` в конец команды установки. Например, версия `v1.0.0`:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/install.sh) v1.0.0
```

## Ручная установка

### Linux/macOS

1. Скачайте последнюю версию S-UI для вашей системы и архитектуры из GitHub: [https://github.com/deposist/s-ui-rus-inst/releases/latest](https://github.com/deposist/s-ui-rus-inst/releases/latest)
2. **Необязательно:** скачайте последнюю версию `s-ui.sh`: [https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/s-ui.sh](https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/s-ui.sh)
3. **Необязательно:** скопируйте `s-ui.sh` в `/usr/bin/` и выполните `chmod +x /usr/bin/s-ui`.
4. Распакуйте tar.gz-архив s-ui в выбранный каталог и перейдите в распакованную папку.
5. Скопируйте файлы `*.service` в `/etc/systemd/system/`, затем выполните `systemctl daemon-reload`.
6. Выполните `systemctl enable s-ui --now`, чтобы включить автозапуск и запустить службу S-UI.
7. Выполните `systemctl enable sing-box --now`, чтобы запустить службу sing-box.

### Windows

1. Скачайте последнюю версию для Windows из GitHub: [https://github.com/deposist/s-ui-rus-inst/releases/latest](https://github.com/deposist/s-ui-rus-inst/releases/latest)
2. Скачайте подходящий пакет для Windows, например `s-ui-windows-amd64.zip`.
3. Распакуйте ZIP-файл в выбранный каталог.
4. Запустите `install-windows.bat` от имени администратора.
5. Следуйте инструкциям мастера установки.
6. Откройте панель: http://localhost:2095/app

## Удаление S-UI

```sh
sudo -i

systemctl disable s-ui  --now

rm -f /etc/systemd/system/sing-box.service
systemctl daemon-reload

rm -fr /usr/local/s-ui
rm /usr/bin/s-ui
```

## Установка с помощью Docker

<details>
   <summary>Показать подробности</summary>

### Использование

**Шаг 1:** установите Docker

```shell
curl -fsSL https://get.docker.com | sh
```

**Шаг 2:** установите S-UI

> Вариант с Docker Compose

```shell
services:
  s-ui:
    image: ghcr.io/deposist/s-ui-rus-inst
    container_name: s-ui
    hostname: "s-ui"
    network_mode: host
    volumes:
      - "./db:/app/db"
      - "./cert:/app/cert"
    tty: true
    restart: unless-stopped
    entrypoint: "./entrypoint.sh"
```

`docker compose up -d`

> Прямой запуск через Docker

```shell
mkdir s-ui && cd s-ui

docker run -itd \
    --network host \
    -v $PWD/db/:/app/db/ \
    -v $PWD/cert/:/root/cert/ \
    --name s-ui \
    --restart=unless-stopped \
    ghcr.io/deposist/s-ui-rus-inst
```

> Самостоятельная сборка образа

```shell
git clone https://github.com/deposist/s-ui-rus-inst
docker build -t s-ui .
```

</details>

## Ручной запуск для разработки и участия в проекте

<details>
   <summary>Показать подробности</summary>

### Сборка и запуск полного проекта

```shell
./runSUI.sh
```

### Клонирование репозитория

```shell
# Клонирование репозитория
git clone https://github.com/deposist/s-ui-rus-inst
```

### Фронтенд

Код фронтенда находится в каталоге [frontend](frontend).

### Бэкенд

> Перед сборкой бэкенда нужно хотя бы один раз собрать фронтенд.

Сборка бэкенда:

```shell
# Удаление старых собранных файлов фронтенда
rm -fr web/html/*
# Копирование новых собранных файлов фронтенда
cp -R frontend/dist/ web/html/
# Сборка
go build -o sui main.go
```

Запуск бэкенда из корня репозитория:

```shell
./sui
```

</details>

## Языки

- Английский
- Персидский
- Вьетнамский
- Упрощенный китайский
- Традиционный китайский
- Русский

## Возможности

- Поддерживаемые протоколы:
  - Общие протоколы: Mixed, SOCKS, HTTP, HTTPS, Direct, Redirect, TProxy
  - Протоколы на базе V2Ray: VLESS, VMess, Trojan, Shadowsocks
  - Другие протоколы: ShadowTLS, Hysteria, Hysteria2, Naive, TUIC
- Поддержка протокола XTLS.
- Продвинутый интерфейс маршрутизации трафика с поддержкой PROXY Protocol, External, прозрачного прокси, SSL-сертификатов и настройки портов.
- Продвинутый интерфейс настройки входящих и исходящих подключений.
- Поддержка лимита трафика и срока действия для клиентов.
- Отображение онлайн-клиентов, статистики трафика входящих и исходящих подключений, а также мониторинг состояния системы.
- Служба подписок поддерживает добавление внешних ссылок и подписок.
- Web-панель и служба подписок поддерживают безопасный доступ по HTTPS (необходимо самостоятельно предоставить домен и SSL-сертификат).
- Темная/светлая тема.

## Переменные окружения

<details>
  <summary>Показать подробности</summary>

### Использование

| Переменная | Тип | Значение по умолчанию |
| -------------- | :--------------------------------------------: | :------------ |
| SUI_LOG_LEVEL | `"debug"` \| `"info"` \| `"warn"` \| `"error"` | `"info"` |
| SUI_DEBUG | `boolean` | `false` |
| SUI_BIN_FOLDER | `string` | `"bin"` |
| SUI_DB_FOLDER | `string` | `"db"` |
| SINGBOX_API | `string` | - |
| SUI_TRUSTED_PROXIES | список CIDR/IP через запятую | - (XFF игнорируется) |
| SUI_ALLOW_PRIVATE_SUB_URLS | `boolean` | `false` |
| SUI_SECRETBOX_KEY | `string` | - (fallback на `settings.secret`) |

</details>

## SSL-сертификаты

<details>
  <summary>Показать подробности</summary>

### Certbot

```bash
snap install core; snap refresh core
snap install --classic certbot
ln -s /snap/bin/certbot /usr/bin/certbot

certbot certonly --standalone --register-unsafely-without-email --non-interactive --agree-tos -d <ваш домен>
```

</details>

#### Благодарность автору оригинального проекта: alireza0

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=deposist/s-ui-rus-inst&type=date&theme=dark" />
  <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=deposist/s-ui-rus-inst&type=date" />
  <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=deposist/s-ui-rus-inst&type=date" />
</picture>
