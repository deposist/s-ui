# Ревью кода `s-ui-rus-inst` и план устранения дефектов

Документ — результат архитектурного ревью репозитория без запуска инструментов
(`go vet`, `go test -race`, `gosec`, `npm run lint`). Под каждый пункт приведена
ссылка на файл и строку. Прогон линтеров и тестов оформлен отдельной задачей и
выполняется в Code-режиме перед началом правок.

Обозначения приоритетов:

- **P0** — критично, исправляем в первую очередь (потенциальная RCE/эскалация
  привилегий, повреждение БД, race-условия в горячем пути, утечка данных).
- **P1** — важная безопасность/устойчивость, исправляем в течение релизного цикла.
- **P2** — баги/корнер-кейсы и архитектурный долг.
- **P3** — стиль, мелочи, возможная замена устаревших пакетов.

---

## 1. Критичные находки (P0)

### C1. Гонка и потерянная ошибка в [`PanelService.RestartPanel`](../service/panel.go:15)

```go
err = p.Kill()      // присваивание захваченной переменной из горутины
err = p.Signal(...) // та же история
if err != nil { logger.Error(...) }
```

Внешняя `err` пишется/читается из разных горутин без синхронизации.
Аналогичный паттерн в [`database.SendSighup`](../database/backup.go:379).
Кроме того, в проекте уже существует [`service.defaultRestartManager`](../service/restart_manager.go:16),
который умеет в координацию — рядом два механизма перезапуска, отсюда и гонки.

**Фикс:** локальная переменная внутри горутины + использовать
`restart_manager.go` как единственный путь.

### C2. Стат-трекер теряет счётчики при `Reset` ([`core/tracker_stats.go`](../core/tracker_stats.go:38))

`Reset()` пересоздаёт мапы, но активные `Counter` всё ещё держат указатели на
старые `*atomic.Int64` через [`bufio.NewInt64CounterConn`](../core/tracker_stats.go:85).
Старые соединения после рестарта core пишут в осиротевшие счётчики — потеря
трафика и микро-утечка памяти.

**Фикс:** в `Reset` обнулять `Counter.read/write` через `Store(0)` вместо
пересоздания мап, либо протащить «эпоху» и игнорировать запись в старую.

### C3. Race в [`web/session_store.go`](../web/session_store.go:30) — общий `*Options`

[`Options()`](../web/session_store.go:51) ассигнит `s.options` без блокировки.
Указатель шарится между запросами; смена `Secure` во время активного запроса
даёт data race.

**Фикс:** `sync.RWMutex` вокруг `s.options`, либо копия per-request.

### C4. SSRF — пропускаются IANA-зарезервированные диапазоны и IPv4-mapped IPv6

[`util/ssrf/validator.go`](../util/ssrf/validator.go:127) опирается на
`addr.IsPrivate()`, в котором отсутствуют:

- `0.0.0.0/8`, `192.0.0.0/24`, `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`;
- `169.254.169.254` (cloud metadata, AWS/GCP/Azure);
- IPv4-mapped IPv6 (`::ffff:127.0.0.1`).

В режиме Telegram-прокси и `validateOutboundCheckTarget` — путь к metadata-эндпоинтам.

**Фикс:** расширить `blockedPrefixes` IANA-зарезервированными диапазонами,
проверять `addr.Unmap()`, и явно блокировать `169.254.169.254/32`.

### C5. Симлинк-атака в [`validateRollbackPath`](../api/import_xui.go:402)

`filepath.Abs` не разрешает симлинки. Атакующий, имеющий права записи в `dbFolder`
(обычно `root`/`sudo`), может создать симлинк
`s-ui-pre-xui-import-X.db -> /etc/passwd` и через rollback подменить базу
на произвольный файл — эскалация локально.

**Фикс:** `filepath.EvalSymlinks(abs)` перед сравнением + `os.Lstat`-проверка
обычного файла + ограничение размера и возраст файла.

### C6. Канонизация IP не выполняется

[`api/utils.go::splitRemoteIP`](../api/utils.go:49) возвращает «как есть».
IPv4-mapped IPv6 (`::ffff:1.2.3.4`) и `1.2.3.4` считаются разными ключами в
rate-limit-мапах и проверках trusted proxies. Атакующий, бьющий по IPv6-листенеру,
обходит per-IP-рейт-лимит.

**Фикс:** хелпер `canonicalIP(value string) string` через
`netip.ParseAddr().Unmap().String()`, использовать везде:
[`getRemoteIp`](../api/utils.go:26), [`isTrustedProxy`](../api/utils.go:102),
ключи в [`rateLimit.go`](../api/rateLimit.go), [`sub/rate_limit.go`](../sub/rate_limit.go:38).

### C7. Утечка горутины в [`auditWriter`](../service/audit_writer.go:141)

После `StopAuditWriter` создаётся новый writer без `Start()` — пока не
произошёл первый `Enqueue`, очередь не читается. На SIGHUP-перезапуске возможна
потеря initial-аудит-событий.

**Фикс:** в `StopAuditWriter` после пересоздания `defaultAuditQueue` сразу
вызывать `defaultAuditQueue.Start()`.

### C8. `bumpVersionSetting` стирает downgrade-инфо

[`database/adapt.go::bumpVersionSetting`](../database/adapt.go:76) перезаписывает
`settings.version` без проверки, что новая версия ≥ старой. Даунгрейд бинарника
(1.5.2 → 1.5.1) молча обнулит метку, при следующем апгрейде миграции пробегут
повторно — `to1_5` имеет UPDATE-запросы по `client_ips`, при определённых данных
это перестирает уже посчитанные `ip_hash`.

**Фикс:** проверка `versionIsNewer(current, dbVersion) || equal`, иначе только
лог.

---

## 2. Высокая важность (P1)

### H1. Бесполезные ветки `default ↔ stored` для `secret`/`installSalt`

[`service/setting.go:282`](../service/setting.go:282) и
[`service/setting.go:293`](../service/setting.go:293) — сравнение
`secret == defaultValueMap["secret"]` всегда ложь после первой записи (значения
рандомные на старте процесса). Логика «если значение из БД равно дефолту,
сохраним» бессмысленна.

**Фикс:** заменить на явный путь: `if database.IsNotFound(err) { create }`.

### H2. Один секрет используется и для cookie HMAC, и для secretbox

[`service/secret_settings.go::getSecretbox`](../service/secret_settings.go:42)
падает в `GetSecret()`, который применяется как cookie-HMAC ключ
([`web/web.go:88`](../web/web.go:88)). Анти-паттерн: компрометация одного
ключа открывает оба домена. HKDF-context фиксирован
(`info=[]byte("settings secrets")`) и недостаточен.

**Фикс:** ввести два явно различных KDF-derived ключа: `cookieKey` и
`settingsKey`, или хотя бы две разные `info` HKDF + поддержка ротации
через `SUI_COOKIE_KEY` env.

### H3. CSRF не привязан к id сессии

[`api/csrf.go::IssueCSRFToken`](../api/csrf.go:22) генерирует токен на
сессию. После `RotateSessionGeneration` (`/api/logoutAllAdmins`) не происходит
сброса `csrfTokenKey` в действующих сессиях — stale-токен может пережить
ротацию. То же про login: при перелогине CSRF-токен не обнуляется насильно.

**Фикс:** при login и при `RotateSessionGeneration` — `session.Delete(csrfTokenKey)`.

### H4. WS-токен timing-leak

[`consumeWSToken`](../api/realtime.go:321) хранит токены как ключ map. Map-lookup
не constant-time. Хранить `sha256(token)`, сверять через
`subtle.ConstantTimeCompare`.

### H5. `telegramNotifier` без `Stop`

[`telegramNotifier.run`](../service/telegram.go:148) — бесконечный цикл, без
обработки `Stop`. На SIGHUP-перезапуске старый цикл продолжает писать audit через
`database.GetDB()`, который уже мог быть переоткрыт через ImportDB.

**Фикс:** ввести `Stop(ctx) error` по аналогии с `auditWriter`, дёргать в
`app.Stop()`.

### H6. `sub/rate_limit.go` без GC

[`rateLimitBuckets`](../sub/rate_limit.go:27) пополняется бесконечно. DoS через
память по большому количеству IP.

**Фикс:** скопировать `gcLoginRateLimitsLocked` style + cap из
[`api/rateLimit.go`](../api/rateLimit.go).

### H7. Сессионный cookie `Secure` зависит от `requestIsHTTPS`

[`api/session.go:24`](../api/session.go:24): `Secure: requestIsHTTPS(c)`. Если
панель за reverse-proxy без `SUI_TRUSTED_PROXIES`, `Secure` всегда `false`. На
HTTPS-сайте cookie уйдёт в plain.

**Фикс:** настройка `forceCookieSecure` или авто-`Secure`, если `webDomain` или
`webURI` начинаются с `https`.

### H8. `service/setting.go::Save` обновляет любые ключи без allow-list

[`SettingService.Save`](../service/setting.go:545) обновляет всё, что прислал
клиент. Любой администратор/токен с scope `admin` может через
`/api/save object=settings data={"installSalt":"..."}` инвалидировать API-токены и
все `ip_hash`.

**Фикс:** explicit allow-list ключей в `SettingService.Save`, всё остальное
отвергается.

### H9. `audit` rate-limit per-actor вместо per-actor+IP

[`api/apiFoundation.go::enforceAuditEndpointRateLimit`](../api/apiFoundation.go:178)
ключует только `actor` (username). Два администратора с одного IP делят бюджет;
угнанная сессия имеет «свой» бюджет.

**Фикс:** ключ `actor + "|" + canonicalIP`.

---

## 3. Средняя важность (P2)

### M1. После `ImportDB` не сбрасываются in-memory кеши

[`database.ImportDB`](../database/backup.go:190) переоткрывает БД, но не
сбрасывает:

- [`ipmonitor.ipHashSalt`](../ipmonitor/ipmonitor.go:69) — старые `ip_hash` из
  кеша не сматчатся → enforce-mode заблокирует всех клиентов.
- [`service/observability.go::observabilityMemoryCapCache`](../service/observability.go:72) — лимит может ссылаться на старую конфигурацию.
- [`api/realtime.go::wsTokens`](../api/realtime.go:45) — токены админов из
  предыдущей БД останутся валидными до TTL.
- [`api/import_xui.go::xuiRates`](../api/import_xui.go:49).
- [`service/update.go::versionCheckState`](../service/update.go:39).

**Фикс:** ввести `database.ResetCaches()` или коллекцию пакетных хуков
`OnDatabaseReinitialized`, дёргать перед `InitDB` после ImportDB.

### M2. `core.Reset` оставляет ConnTracker в работе после `Stop`

В [`core/box.go::Close`](../core/box.go:479) дёргается
`statsTracker.Reset()` и `connTracker.Reset()`, но wrapped-conn'ы продолжают
держать ссылку на `tracker`. Их `Read/Write` после Reset делает `untrackConnection`
по уже очищенной мапе — корректно, но ускользает от инварианта «после `Close`
объект больше не используется».

**Фикс:** контракт: `tracker.Reset` блокирует `Reset` до завершения активных
горутин (`sync.WaitGroup` на `wrapped*`).

### M3. `database/db.go` без `_foreign_keys=on`

В DSN ([`database/db.go:75`](../database/db.go:75)) отсутствует прагма
`_foreign_keys=on`. FK-связи в схеме объявлены через GORM-теги, но SQLite их
не enforce-ит. Прямое удаление через cli оставляет «висящие» tokens/clients.

**Фикс:** добавить `&_foreign_keys=on` в DSN. Проверить, не сломаются ли
существующие данные (тогда нужно `PRAGMA foreign_key_check;` в миграции).

### M4. Race на `wsPingInterval` / `wsPingTimeout`

[`api/realtime.go:36`](../api/realtime.go:36) — `var`-глобалы, тесты их меняют.
`go test -race ./api/...` это поймает.

**Фикс:** `const` + параметризовать heartbeat через структуру или ввести
`WithPingInterval` для тестов.

### M5. `import_xui.go::ImportXuiApply` лимитирует FormData полем 1 MiB

[`api/import_xui.go:315`](../api/import_xui.go:315) `io.LimitReader(part, 1<<20)`.
Для больших 3x-ui панелей plan может быть >1 MiB, тогда декодинг возвращает
«invalid character» вместо человеческой ошибки.

**Фикс:** поднять лимит до 8 MiB, либо вернуть `payload_too_large` при усечении
конкретного поля.

### M6. `RotateSessionGeneration` закрывает все realtime-WS, но не вызывает CSRF-инвалидацию

[`service/setting.go::RotateSessionGeneration`](../service/setting.go:311) делает
`realtime.CloseAll`, но не зачищает CSRF-токены сессий и не сбрасывает
`api/realtime.wsTokens`, поэтому ws-token'ы выданные в момент ротации могут
переиспользоваться (TTL 60s).

**Фикс:** в `RotateSessionGeneration` дополнительно зачищать `wsTokens`
(`sweepAllWSTokensLocked`).

### M7. SQL: динамическая склейка `inboundType` в `addUsers`

[`service/inbounds.go::fetchUsersByCondition`](../service/inbounds.go:339) вшивает
`field` в `json_extract(...,'$.shadowsocks')` через `fmt.Sprintf`. Сейчас защита
строится на allow-list `userJSONField`. Это ОК, но в коде следует оставить
явный комментарий-«TODO: do not extend without a positive list».

**Фикс:** сейчас не баг, но прецедент SQLi — задокументировать инвариант,
добавить unit-тест на негативные значения.

### M8. `service/client.go::ResetClients` использует `Save` (full update)

При сбросе клиенту перезаписываются **все** колонки. Если внешняя ручка только
что обновила `Volume`/`Expiry`, реcет затрёт. Маловероятно, но возможно.

**Фикс:** `Updates(map[string]interface{}{...})` со списком колонок в третьем
блоке.

### M9. `core.NewCore` модифицирует процессно-глобальный `globalCtx`

[`core/main.go::setGlobalCtx`](../core/main.go:141) — пакетная переменная,
тестируемость и race-устойчивость низкие. Для основного потока ОК, но при
параллельных тестах и в будущем при поддержке multi-instance — head-ache.

**Фикс:** перенести `ctx` внутрь `Core`, удалить пакетную переменную.

### M10. Frontend: токен в `Sec-WebSocket-Protocol`

[`frontend/src/store/ws.ts:215`](../frontend/src/store/ws.ts:215) шлёт токен через
subprotocol. На бэкенде [`api/realtime.go::wsTokenFromRequest`](../api/realtime.go:219)
берёт первый non-`sui.realtime` элемент. Лучше иметь явный префикс
(`sui.token.<base64>`), чтобы не зависеть от порядка и расширяемости.

### M11. `Telegram` SOCKS5-dialer утекает горутины при таймауте

[`service/telegram.go:505`](../service/telegram.go:505) — `go func() { dialer.Dial }`.
Если внешний контекст закрылся, горутина продолжает блокироваться в `Dial` пока
не отвалится TCP-таймаут операционки.

**Фикс:** dial с явным `net.Dialer{Timeout}`-аналогом или ограничением
«самоотмены» через таймер; иначе на флапе провайдера утечёт пул.

### M12. SSRF-валидатор IPv6-резерв

[`util/ssrf/validator.go::isBlockedAddr`](../util/ssrf/validator.go:126) не
блокирует IPv6 ULA-resvd `fc00::/7` за пределами `IsPrivate()`, и **не**
блокирует `2001:db8::/32` (документация), `64:ff9b::/96` (NAT64). Сильно
теоретически, но добавление списка не помешает.

### M13. `service/telegram_backup.go::RunOnce` — лишний `bytes.Equal` префикса

[`service/telegram_backup.go:140`](../service/telegram_backup.go:140) — sanity
check бесполезен, GCM с разными nonce никогда не совпадёт по префиксу с plain.
Тратит CPU на больших backup'ах.

**Фикс:** удалить или заменить на `len(envelope) > len(payload)+overhead`.

---

## 4. Низкая важность (P3)

- **L1.** [`api/utils.go::checkLogin`](../api/utils.go:170) делает
  `c.Redirect(307, "./login")`. Лучше абсолютный путь от `webPath`.
- **L2.** [`util/common/err.go::NewError`](../util/common/err.go:16) добавляет
  `\n` в конец сообщения, что портит API-ответы (`jsonMsgObj` склеивает через
  `+ ": "`). Косметика.
- **L3.** [`service/telegram.go::SendTelegramDocument`](../service/telegram.go:268)
  держит весь backup в `bytes.Buffer` — копия. Лучше pipe (`io.Pipe`).
- **L4.** [`config/config.go::GetLogLevel`](../config/config.go:35) пускает любые
  значения `SUI_LOG_LEVEL`, в `app.initLog` падает с `log.Fatal`. Должна быть
  валидация-fallback на `info` с warning'ом.
- **L5.** [`web/web.go:94`](../web/web.go:94) — при `webPath = "/"` ассеты
  попадают на `/assets/`, что блокирует [`util.ReservedPathPrefixes`](../util/path_validate.go:9)
  для `subPath`. Нечеткое сообщение об ошибке. Документировать.
- **L6.** [`frontend/src/views/Login.vue:90`](../frontend/src/views/Login.vue:90)
  `setTimeout(..., 500)` после успешного логина — артефакт, удалить.
- **L7.** [`service/client.go:443`](../service/client.go:443) сравнивает int64
  трафик; теоретическое переполнение через ~9 EB.
- **L8.** [`util/redact/redact.go`](../util/redact/redact.go) паттерн
  `[A-Z2-7]{32}` — потенциальные false-positives на base32-хешах. Не критично.
- **L9.** [`cronjob/xuiSyncJob.go:75`](../cronjob/xuiSyncJob.go:75) — `time.After`
  в цикле вместо `NewTimer` (мусорный таймер).
- **L10.** [`database/db.go:60`](../database/db.go:60) `MaxOpenConns(8)` без
  `_synchronous=NORMAL` — write-tx через fsync. Performance.
- **L11.** Зависимость `github.com/op/go-logging` устарела; миграция на
  `log/slog` (стандарт ≥ Go 1.21) — отдельный отрефакторенный задел.
- **L12.** [`service/setting.go:112`](../service/setting.go:112) — `version`
  фиксируется в `defaultValueMap` на момент init. В тестах с подменой версии —
  нелинейность.
- **L13.** [`api/import_xui_remote.go::syncSafeHostPort`](../api/import_xui_remote.go:284)
  вырезает протокол строкой; для аудита достаточно `url.Parse`.

---

## 5. Архитектурный долг

1. **Два механизма перезапуска** ([`service/panel.go`](../service/panel.go) и
   [`service/restart_manager.go`](../service/restart_manager.go)) расходятся.
   `service/panel.go` старее, `restart_manager.go` — каноничный. Удалить
   `PanelService.RestartPanel` или сделать тонкой обёрткой над `defaultRestartManager.sendSighup`.
2. **Глобальные пакетные переменные** в [`core/main.go`](../core/main.go),
   [`service/config.go::corePtr/lastUpdateMu/LastUpdate`](../service/config.go:19),
   [`service/telegram.go::defaultTelegramNotifier`](../service/telegram.go:42),
   [`service/audit_writer.go::defaultAuditQueue`](../service/audit_writer.go:22) — каждая
   добавляет блокеры тестируемости и race-риски. Постепенно заменять на
   зависимости, передаваемые через DI/конструкторы.
3. **`webListen`/`subListen` fallback на любой интерфейс** реализован в
   [`network/listen.go`](../network/listen.go:18), но без аудита (не пишет
   `audit_event` о факте fallback). Полезно знать в логах ИБ.
4. **Обёртка `connTracker`** дублирует поведение sing-box-овой цепочки трекеров.
   Если sing-box обновляется, нужно ревалидировать корректность.
5. **Vue/Vite** на 8.x с TypeScript 6 + ESLint 10 — `eslint-plugin-vue` 10.8
   совместим, но прогон линтера обязателен.

---

## 6. План работ (этапы)

### Этап 0 — Подготовка (Code-режим)

- [ ] Прогнать `go mod tidy && go build ./...` (фиксируем текущее состояние).
- [ ] Прогнать `go vet ./...`, `golangci-lint run` (gocritic, govet, ineffassign,
      staticcheck, gosec).
- [ ] Прогнать `go test -race ./...` (потребует CGO для sqlite).
- [ ] В каталоге `frontend/` прогнать `npm install`, `npm run lint`, `npm run test:unit`,
      `npm run build`.
- [ ] Зафиксировать вывод инструментов в `plans/lint-baseline.txt`.

### Этап 1 — P0 (1 PR, security/race)

- [x] **C1** — переписать `PanelService.RestartPanel` через `restart_manager.go`,
      убрать data race; либо полностью заменить вызовы на `defaultRestartManager.sendSighup`.
      Исправлено локальными переменными в горутинах; `go test -race ./service ./database ./api` PASS.
- [x] **C2** — `StatsTracker.Reset` без пересоздания мап.
      `go test -race ./core` PASS.
- [x] **C3** — `SQLiteSessionStore` мьютекс на `s.options`.
      `go test -race ./web ./api` PASS.
- [x] **C4** — расширить `blockedPrefixes` в [`util/ssrf/validator.go`](../util/ssrf/validator.go),
      `addr.Unmap()`, +тесты на metadata-эндпоинты.
      `go test ./util/ssrf` PASS.
- [x] **C5** — `EvalSymlinks` + `Lstat` в [`api/import_xui.go::validateRollbackPath`](../api/import_xui.go:402),
      +тест с симлинком.
      `go test ./api -run "Xui|Rollback|ValidateRollback"` PASS.
- [x] **C6** — `canonicalIP` хелпер; миграция в [`api/utils.go`](../api/utils.go),
      [`api/rateLimit.go`](../api/rateLimit.go), [`sub/rate_limit.go`](../sub/rate_limit.go).
      Реализовано в `api/utils.go` и `sub/rate_limit.go`; `api/rateLimit.go` уже получает IP через `getRemoteIp`.
      `go test ./api ./sub ./realtime` PASS.
- [x] **C7** — `Start()` на пересоздаваемый writer в `StopAuditWriter`.
      Реализован lifecycle guard: idempotent `Start`/`Stop`, enqueue-after-stop rejected.
      `go test -race ./service -run Audit` PASS.
- [x] **C8** — guard на даунгрейд в `bumpVersionSetting`.
      `go test ./database -run "Adapt|Version"` PASS.
- [x] Тесты: `service`/`api`/`util/ssrf`/`database` — покрыть кейсы, прогнать
      `-race`.
      Финальный backend: `go test -race ./core ./web ./service ./database ./database/importxui ./api`, `go vet ./...`, `go build ./...` PASS.
- [ ] Audit: записать события для каждого нового защитного пути (`ssrf_blocked`,
      `rollback_path_rejected`). Deferred: не добавлено в P0-pass, чтобы не расширять API/журналирование сверх минимальных security/race-фиксов.
+
+Статус P0-pass: fixed. Validation log: [`plans/fix-validation.txt`](fix-validation.txt).
+External/deferred:
+
+- External baseline race: `github.com/sagernet/sing-tun` Windows monitor race, без project stack.
+- Baseline `database/importxui` TempDir cleanup: targeted rerun PASS, проектная утечка handle не воспроизведена.
+- Frontend `npm test`: FAIL до выполнения тестов из-за Vitest/Vite/Node runtime issue `Cannot read properties of undefined (reading 'config')`; `npm run lint` и `npm run build` PASS.

### Этап 2 — P1 (2-й PR)

- [ ] **H1** — упрощённый `GetSecret`/`GetInstallSalt` (через `IsNotFound`).
- [ ] **H2** — KDF-разделение ключей cookie ↔ secretbox; задокументировать
      `SUI_COOKIE_KEY`, `SUI_SECRETBOX_KEY` в `docs/scope-matrix.md`.
- [ ] **H3** — сброс CSRF-токена при login и `RotateSessionGeneration`.
- [ ] **H4** — sha256-хранение WS-токенов + `ConstantTimeCompare`.
- [ ] **H5** — `telegramNotifier.Stop`, вызов в `app.Stop`.
- [ ] **H6** — GC + cap для `sub/rate_limit.go`.
- [ ] **H7** — `forceCookieSecure` + автодетект по `webDomain`/`webURI`.
- [ ] **H8** — allow-list ключей в `SettingService.Save`.
- [ ] **H9** — ключ rate-limit `actor + canonicalIP` для audit.
- [ ] Юнит-тесты, обновить `docs/scope-matrix.md`.

### Этап 3 — P2 (3-й PR, надёжность)

- [ ] **M1** — `database.ResetCaches()` + хуки в `ipmonitor`, `observability`,
      `wsTokens`, `xuiRates`, `versionCheckState`.
- [ ] **M2** — `WaitGroup` в трекерах.
- [ ] **M3** — `_foreign_keys=on` в DSN, миграция-аудит существующих данных.
- [ ] **M4** — `wsPingInterval`/`wsPingTimeout` через структуру (для тестов).
- [ ] **M5** — лимит plan-поля 8 MiB + понятная ошибка.
- [ ] **M6** — `wsTokens.sweepAll()` в `RotateSessionGeneration`.
- [ ] **M7** — комментарий-инвариант + тест в `service/inbounds.go`.
- [ ] **M8** — `Updates(map)` в `ResetClients`.
- [ ] **M9** — выноc `globalCtx` в `Core`.
- [ ] **M10** — фронтенд-protocol prefix `sui.token.<base64>`.
- [ ] **M11** — таймаут SOCKS5-dial.
- [ ] **M12** — IPv6-prefix-list.
- [ ] **M13** — удалить лишний `bytes.Equal`.

### Этап 4 — P3 (отдельные мелкие PR-ы по мере возможности)

- [ ] **L1**–**L13** — косметика, документация, замена `op/go-logging` на
      `log/slog` (отдельный реляционный мини-проект).

### Этап 5 — Архитектурный долг

- [ ] Слияние `PanelService.RestartPanel` ↔ `restart_manager.go`.
- [ ] Сокращение глобальных пакетных переменных (DI-плюшки, удобный shutdown).
- [ ] Аудит-запись о фолбэке listen-адреса.
- [ ] Регулярный re-validate sing-box trackers при апгрейде `sing-box`.
- [ ] Спецификация версионирования `version` (semver, monotonic guard).

---

## 7. Что хорошего в текущем коде

Чтобы не звучать односторонне:

- В [`service/audit_writer.go`](../service/audit_writer.go) аккуратная
  очередь с backpressure, drop-counter и graceful shutdown.
- В [`api/realtime.go`](../api/realtime.go) реализован
  Origin-allow-list + per-IP/per-user reservations, rate-limit handshake,
  ping/pong heartbeat и close-all on session rotate.
- В [`util/secretbox/secretbox.go`](../util/secretbox/secretbox.go) HKDF + AES-GCM
  с associated data, явно версионированный префикс `sbox:v1:` — задел на
  ротацию криптографии.
- В [`database/backup.go`](../database/backup.go) — staged-import с integrity
  check + откат к `.backup` копии при любом сбое.
- В [`ipmonitor/ipmonitor.go`](../ipmonitor/ipmonitor.go) корректная разница
  monitor/enforce, дебаунс `security_event` и кеш с TTL.
- Frontend WebSocket store ([`frontend/src/store/ws.ts`](../frontend/src/store/ws.ts))
  с экспоненциальным backoff, polling-fallback и `closeFallbackThreshold` —
  пример хорошей деградации.

Этот фундамент позволяет точечно закрывать находки без переписывания
архитектуры. Начинать имеет смысл с этапа 0 (baseline линтеров и тестов),
дальше — этап 1 единым PR. По завершении каждого этапа имеет смысл выпускать
hotfix-релиз с CHANGELOG-EN/RU/ZH.
