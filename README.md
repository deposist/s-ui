## S-UI

## English

Advanced Web panel built on `SagerNet/Sing-Box`.

**Note:** the original `alireza0/s-ui` project was blocked and removed by GitHub. This repository is a complete backup based on the last original version, `v1.4.1`, and contains the full frontend and backend source code.

**This fork keeps the original project structure and updates the user-facing documentation and installation links for this repository. You can use the scripts from this repository directly, or fork and build the project yourself.**

> **Disclaimer:** this project is intended only for personal learning and knowledge sharing. Do not use it for illegal purposes.


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
- Username/password: admin

## Install or Upgrade to the Latest Version

### Linux/macOS

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui/main/install.sh)
```

### Windows

1. Download the latest Windows version from [GitHub Releases](https://github.com/deposist/s-ui/releases/latest).
2. Extract the ZIP file.
3. Run `install-windows.bat` as Administrator.
4. Follow the installation wizard.

## Install an Older Version

**Step 1:** to install a specific older version, append the version tag with `v` to the installation command. For example, version `v1.0.0`:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui/main/install.sh) v1.0.0
```


## Manual Installation

### Linux/macOS

1. Download the latest S-UI version for your system and architecture from GitHub: [https://github.com/deposist/s-ui/releases/latest](https://github.com/deposist/s-ui/releases/latest)
2. **Optional:** download the latest `s-ui.sh`: [https://raw.githubusercontent.com/deposist/s-ui/main/s-ui.sh](https://raw.githubusercontent.com/deposist/s-ui/main/s-ui.sh)
3. **Optional:** copy `s-ui.sh` to `/usr/bin/` and run `chmod +x /usr/bin/s-ui`.
4. Extract the s-ui tar.gz archive to your chosen directory and enter the extracted folder.
5. Copy the `*.service` files to `/etc/systemd/system/`, then run `systemctl daemon-reload`.
6. Run `systemctl enable s-ui --now` to enable autostart and start the S-UI service.
7. Run `systemctl enable sing-box --now` to start the sing-box service.

### Windows

1. Download the latest Windows version from GitHub: [https://github.com/deposist/s-ui/releases/latest](https://github.com/deposist/s-ui/releases/latest)
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
    image: ghcr.io/deposist/s-ui
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
    ghcr.io/deposist/s-ui
```

> Build the image yourself

```shell
git clone https://github.com/deposist/s-ui
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
git clone https://github.com/deposist/s-ui
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

**Примечание:** оригинальный проект `alireza0/s-ui` был заблокирован и удален GitHub. Этот репозиторий является полной резервной копией последней оригинальной версии `v1.4.1` и содержит полный исходный код фронтенда и бэкенда.

**Этот fork сохраняет структуру оригинального проекта и обновляет пользовательскую документацию и ссылки установки для этого репозитория. Вы можете напрямую использовать скрипты из этого репозитория или сделать fork и собрать проект самостоятельно.**

> **Отказ от ответственности:** этот проект предназначен только для личного обучения и обмена опытом. Не используйте его в незаконных целях.


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
- Имя пользователя/пароль: admin

## Установка или обновление до последней версии

### Linux/macOS

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui/main/install.sh)
```

### Windows

1. Скачайте последнюю версию для Windows из [GitHub Releases](https://github.com/deposist/s-ui/releases/latest).
2. Распакуйте ZIP-файл.
3. Запустите `install-windows.bat` от имени администратора.
4. Следуйте инструкциям мастера установки.

## Установка старой версии

**Шаг 1:** чтобы установить определенную старую версию, добавьте тег версии с `v` в конец команды установки. Например, версия `v1.0.0`:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/deposist/s-ui/main/install.sh) v1.0.0
```


## Ручная установка

### Linux/macOS

1. Скачайте последнюю версию S-UI для вашей системы и архитектуры из GitHub: [https://github.com/deposist/s-ui/releases/latest](https://github.com/deposist/s-ui/releases/latest)
2. **Необязательно:** скачайте последнюю версию `s-ui.sh`: [https://raw.githubusercontent.com/deposist/s-ui/main/s-ui.sh](https://raw.githubusercontent.com/deposist/s-ui/main/s-ui.sh)
3. **Необязательно:** скопируйте `s-ui.sh` в `/usr/bin/` и выполните `chmod +x /usr/bin/s-ui`.
4. Распакуйте tar.gz-архив s-ui в выбранный каталог и перейдите в распакованную папку.
5. Скопируйте файлы `*.service` в `/etc/systemd/system/`, затем выполните `systemctl daemon-reload`.
6. Выполните `systemctl enable s-ui --now`, чтобы включить автозапуск и запустить службу S-UI.
7. Выполните `systemctl enable sing-box --now`, чтобы запустить службу sing-box.

### Windows

1. Скачайте последнюю версию для Windows из GitHub: [https://github.com/deposist/s-ui/releases/latest](https://github.com/deposist/s-ui/releases/latest)
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
    image: ghcr.io/deposist/s-ui
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
    ghcr.io/deposist/s-ui
```

> Самостоятельная сборка образа

```shell
git clone https://github.com/deposist/s-ui
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
git clone https://github.com/deposist/s-ui
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
