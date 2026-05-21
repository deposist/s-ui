# Plan: Upstream Issue #1114 (TUIC `udp_relay_mode` in subscription links)

## Goal
Fix upstream bug parity for [#1114](https://github.com/alireza0/s-ui/issues/1114): generated TUIC subscription/share links must include `udp_relay_mode`.

## Scope
- Target generator: `util/genLink.go` (`tuicLink`)
- Parser already supports field: `util/linkToJson.go` (`tuicToJson` / `GetOutbound` path)
- Side-check: `sub/clashService.go` (no regressions in Clash conversion)

## Execution Plan
1. Confirm current gap in `tuicLink` (no `udp_relay_mode` in query params).
2. Add `udp_relay_mode` emission in `tuicLink`:
   - source from inbound fields;
   - if absent, use project-safe default (`quic`, unless repo policy dictates otherwise);
   - skip empty values.
3. Keep generator/parser behavior consistent (generate -> parse round-trip preserves `udp_relay_mode`).
4. Add/extend tests:
   - TUIC generated link contains `udp_relay_mode`;
   - round-trip keeps `udp_relay_mode`.
5. Validate:
   - `go test ./...`
   - `go build ./...`
   - `go test -race ./...` (if feasible)
6. Update changelogs (Unreleased):
   - `CHANGELOG-EN.md`
   - `CHANGELOG-RU.md`
   - `CHANGELOG-ZH.md`

## Definition of Done
- `udp_relay_mode` present in generated TUIC links.
- Tests added and green.
- Build/test validation passed.
- Changelog entries added in EN/RU/ZH.

---

## Prompt for another dialog (copy/paste)
Ссылка на план: `plans/upstream-issue-1114-plan.md`
Ты работаешь в форке `deposist/s-ui-rus-inst`, ветка `beta`.

Нужно актуализировать форк по апстрим-багу [#1114](https://github.com/alireza0/s-ui/issues/1114):
**в TUIC subscription/share links отсутствует `udp_relay_mode`**.

### Проверенный контекст
- Генерация TUIC ссылок: `util/genLink.go`, `tuicLink`.
- Парсинг TUIC уже поддерживает поле `udp_relay_mode`: `util/linkToJson.go` (путь `GetOutbound` для TUIC).
- Побочный контроль регрессий: `sub/clashService.go` (TUIC/Clash conversion).

### Цель
Сделать так, чтобы generated TUIC link **всегда** содержал корректный `udp_relay_mode` (явное значение или безопасный default), без поломки обратной совместимости и других протоколов.

### Что сделать
1. Подтверди дефект в текущем коде (`tuicLink` не добавляет `udp_relay_mode`).
2. Внеси целевой фикс в `util/genLink.go` (`tuicLink`):
   - добавь `udp_relay_mode` в query-параметры;
   - источник — inbound-поле/структура данных;
   - если значение отсутствует — примени безопасный default (`quic`, если нет проектного запрета);
   - пустое значение в ссылку не писать.
3. Проверь консистентность generate -> parse:
   - generated link парсится через `GetOutbound`;
   - `udp_relay_mode` сохраняется и не теряется.
4. Добавь/обнови тесты:
   - тест генерации TUIC ссылки с `udp_relay_mode`;
   - тест round-trip generate->parse для `udp_relay_mode`;
   - тест ветки, где у inbound нет явного режима (default-поведение).
5. Убедись, что не задеты другие протоколы и нет регрессий в TUIC/Clash экспорте (`sub/clashService.go`).
6. Обнови Unreleased changelog (кратко, по одной записи):
   - `CHANGELOG-EN.md`
   - `CHANGELOG-RU.md`
   - `CHANGELOG-ZH.md`

### Валидация (обязательно приложить результаты)
- `go test ./...`
- `go build ./...`
- `go test -race ./...` (если не укладывается по времени — зафиксируй это явно)

### Definition of Done
- В generated TUIC links есть `udp_relay_mode`.
- Тесты на генерацию и round-trip зелёные.
- Сборка и базовые тесты проходят.
- Changelog EN/RU/ZH обновлён.

### Формат итогового отчёта
1. Какие файлы изменены.
2. Почему решение корректно (в т.ч. default-логика).
3. Результаты валидации.
4. Риски/ограничения.
5. Готовый commit message (`subject` + `body`).
