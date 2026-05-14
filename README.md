## S-UI

Продвинутая Web-панель, построенная на базе `SagerNet/Sing-Box`.

**Примечание:** оригинальный проект `alireza0/s-ui` был заблокирован и удален GitHub. Этот репозиторий является полной резервной копией последней оригинальной версии `v1.4.1` и содержит полный исходный код фронтенда и бэкенда.

**В этом репозитории изменены только язык и часовой пояс по умолчанию на китайские. В остальном он соответствует оригинальной версии без изменений. Вы можете напрямую использовать скрипты из этого репозитория или сделать fork и собрать проект самостоятельно.**

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
bash <(curl -Ls https://raw.githubusercontent.com/admin8800/s-ui/main/install.sh)
```

### Windows

1. Скачайте последнюю версию для Windows из [GitHub Releases](https://github.com/admin8800/s-ui/releases/latest).
2. Распакуйте ZIP-файл.
3. Запустите `install-windows.bat` от имени администратора.
4. Следуйте инструкциям мастера установки.

## Установка старой версии

**Шаг 1:** чтобы установить определенную старую версию, добавьте тег версии с `v` в конец команды установки. Например, версия `v1.0.0`:

```sh
bash <(curl -Ls https://raw.githubusercontent.com/admin8800/s-ui/main/install.sh) v1.0.0
```


## Ручная установка

### Linux/macOS

1. Скачайте последнюю версию S-UI для вашей системы и архитектуры из GitHub: [https://github.com/admin8800/s-ui/releases/latest](https://github.com/admin8800/s-ui/releases/latest)
2. **Необязательно:** скачайте последнюю версию `s-ui.sh`: [https://raw.githubusercontent.com/admin8800/s-ui/main/s-ui.sh](https://raw.githubusercontent.com/admin8800/s-ui/main/s-ui.sh)
3. **Необязательно:** скопируйте `s-ui.sh` в `/usr/bin/` и выполните `chmod +x /usr/bin/s-ui`.
4. Распакуйте tar.gz-архив s-ui в выбранный каталог и перейдите в распакованную папку.
5. Скопируйте файлы `*.service` в `/etc/systemd/system/`, затем выполните `systemctl daemon-reload`.
6. Выполните `systemctl enable s-ui --now`, чтобы включить автозапуск и запустить службу S-UI.
7. Выполните `systemctl enable sing-box --now`, чтобы запустить службу sing-box.

### Windows

1. Скачайте последнюю версию для Windows из GitHub: [https://github.com/admin8800/s-ui/releases/latest](https://github.com/admin8800/s-ui/releases/latest)
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
    image: ghcr.io/admin8800/s-ui
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
    ghcr.io/admin8800/s-ui
```

> Самостоятельная сборка образа

```shell
git clone https://github.com/admin8800/s-ui
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
git clone https://github.com/admin8800/s-ui
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
