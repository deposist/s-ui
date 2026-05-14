## S-UI

## English

Advanced Web panel built on `SagerNet/Sing-Box`.

**Note:** the original `alireza0/s-ui` project was blocked and removed by GitHub. This repository is a complete backup based on the last original version, `v1.4.1`, with security and reliability hardening applied on top (current build: `v1.4.2-beta`).

**This fork keeps the original project structure and updates the user-facing documentation and installation links for this repository. You can use the scripts from this repository directly, or fork and build the project yourself.**

> **Disclaimer:** this project is intended only for personal learning and knowledge sharing. Do not use it for illegal purposes.


## What's new in 1.4.2-beta

- bcrypt password storage with automatic plaintext-to-bcrypt migration on first login.
- Random admin password on first install (printed once to the application log).
- Login rate limiter, hardened session cookies, optional `SUI_TRUSTED_PROXIES`.
- SSRF-resistant external subscription fetcher (DNS-rebinding safe, response cap, opt-in private targets via `SUI_ALLOW_PRIVATE_SUB_URLS`).
- Race-free core lifecycle, online-stats and token store.
- Frontend: `v-html` removed from log/IP/rule import surfaces, code splitting re-enabled, Axios `AbortController` instead of deprecated `CancelToken`, ESLint flat config.
- Backup import auto-adapts old databases (1.0/1.1/1.2/1.3/1.4.x) — restoring a legacy backup runs the schema migrations and rehashes any plaintext passwords transparently. Fresh binaries on top of an existing 1.x DB upgrade automatically on start.
- See [CHANGELOG.md](CHANGELOG.md) for the full list and an upgrade guide.


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
- Username: admin
- Password (fresh install only): a random 24-character string is generated on first start and written to the application log. Look for the line `created initial admin user. username=admin password=...` in `journalctl -u s-ui` (Linux) or in the panel log on first run. After that, change it from the panel.

## Install or Upgrade to the Latest Stable Version

### Linux/macOS

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/install.sh)
```

### Windows

1. Download the latest Windows version from [GitHub Releases](https://github.com/deposist/s-ui-rus-inst/releases/latest).
2. Extract the ZIP file.
3. Run `install-windows.bat` as Administrator.
4. Follow the installation wizard.

## Install the 1.4.2 Beta (security/reliability hardening)

To install or upgrade to the current beta build (`v1.4.2-beta`):

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/install.sh) v1.4.2-beta
```

Or from a local clone:

```sh
git clone -b beta https://github.com/deposist/s-ui-rus-inst.git
cd s-ui-rus-inst
sudo bash install.sh v1.4.2-beta
```

The installer is fully compatible with existing installations: settings,
inbounds, outbounds, clients, TLS, services and tokens are kept; the DB
schema is migrated automatically on first start; plaintext admin
passwords are upgraded to bcrypt on the next successful login. See
[CHANGELOG.md](CHANGELOG.md#upgrade-guide--гайд-по-обновлению) for the
full upgrade procedure and rollback notes.

## Install an Older Version

**Step 1:** to install a specific older version, append the version tag with `v` to the installation command. For example, version `v1.0.0`:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/install.sh) v1.0.0
```


## Manual Installation

### Linux/macOS

1. Download the latest S-UI version for your system and architecture from GitHub: [https://github.com/deposist/s-ui-rus-inst/releases/latest](https://github.com/deposist/s-ui-rus-inst/releases/latest)
2. **Optional:** download the latest `s-ui.sh`: [https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/s-ui.sh](https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/s-ui.sh)
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

**Примечание:** оригинальный проект `alireza0/s-ui` был заблокирован и удалён GitHub. Этот репозиторий — полная резервная копия последней оригинальной версии `v1.4.1` с применённым набором исправлений по безопасности и надёжности (текущая сборка: `v1.4.2-beta`).

**Этот fork сохраняет структуру оригинального проекта и обновляет пользовательскую документацию и ссылки установки для этого репозитория. Вы можете напрямую использовать скрипты из этого репозитория или сделать fork и собрать проект самостоятельно.**

> **Отказ от ответственности:** этот проект предназначен только для личного обучения и обмена опытом. Не используйте его в незаконных целях.


## Что нового в 1.4.2-beta

- Хранение паролей через bcrypt с автоматической миграцией plaintext-паролей при первом успешном логине.
- При первой установке генерируется случайный пароль администратора (выводится в журнал приложения один раз).
- Лимит входа, защищённые cookie сессии, опциональный `SUI_TRUSTED_PROXIES`.
- SSRF-устойчивый загрузчик внешних подписок (защита от DNS rebinding, лимит размера, опциональные приватные адреса через `SUI_ALLOW_PRIVATE_SUB_URLS`).
- Race-free жизненный цикл core, онлайн-статистика и хранилище токенов.
- Фронтенд: убран `v-html` из логов/IP-листов/импорта правил, включён code splitting, заменён устаревший `axios.CancelToken` на `AbortController`, ESLint flat config.
- Импорт бэкапа автоматически адаптирует базы старых версий (1.0/1.1/1.2/1.3/1.4.x) — миграция схемы и перешивка plaintext-паролей выполняются прозрачно. Свежий бинарник поверх существующей базы 1.x обновляется автоматически при старте.
- Полный список изменений и руководство по обновлению — в [CHANGELOG.md](CHANGELOG.md).


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
- Имя пользователя: admin
- Пароль (только для свежей установки): при первом запуске генерируется случайная строка из 24 символов, которая выводится в журнал приложения. Найдите строку `created initial admin user. username=admin password=...` в `journalctl -u s-ui` (Linux) или в журнале панели после первого запуска. После входа смените пароль в настройках.

## Установка или обновление до последней стабильной версии

### Linux/macOS

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/install.sh)
```

### Windows

1. Скачайте последнюю версию для Windows из [GitHub Releases](https://github.com/deposist/s-ui-rus-inst/releases/latest).
2. Распакуйте ZIP-файл.
3. Запустите `install-windows.bat` от имени администратора.
4. Следуйте инструкциям мастера установки.

## Установка беты 1.4.2 (исправления безопасности и надёжности)

Установка или обновление до текущей беты (`v1.4.2-beta`):

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/beta/install.sh) v1.4.2-beta
```

Либо из локального клона:

```sh
git clone -b beta https://github.com/deposist/s-ui-rus-inst.git
cd s-ui-rus-inst
sudo bash install.sh v1.4.2-beta
```

Установщик полностью совместим с уже работающими установками: настройки,
inbounds, outbounds, клиенты, TLS, services и токены сохраняются; схема
БД мигрируется автоматически при первом запуске; пароль администратора
в открытом виде заменяется на bcrypt-хеш при следующем успешном входе.
Полный гайд по обновлению и откату — в
[CHANGELOG.md](CHANGELOG.md#upgrade-guide--гайд-по-обновлению).

## Установка старой версии

**Шаг 1:** чтобы установить определенную старую версию, добавьте тег версии с `v` в конец команды установки. Например, версия `v1.0.0`:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/install.sh) v1.0.0
```


## Ручная установка

### Linux/macOS

1. Скачайте последнюю версию S-UI для вашей системы и архитектуры из GitHub: [https://github.com/deposist/s-ui-rus-inst/releases/latest](https://github.com/deposist/s-ui-rus-inst/releases/latest)
2. **Необязательно:** скачайте последнюю версию `s-ui.sh`: [https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/s-ui.sh](https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/s-ui.sh)
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
