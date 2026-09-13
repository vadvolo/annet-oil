# Audit Log — план реализации

Аудит-трейл действий пользователей: кто, когда, на каком оборудовании запускал
команды и annet-операции. Отдельный фронтенд `../annet-web` показывает трейл через
API annet-oil.

## Решения

| Вопрос | Решение |
|---|---|
| Хранилище | **PostgreSQL** (первая БД в проекте — сейчас БД нет вообще) |
| Чтение annet-web | **Через API annet-oil** (`GET /api/v0/audit`), в БД напрямую не ходит |
| Владелец | annet-oil владеет и записью, и чтением |
| Охват | **Все действия**: gen/diff/patch/deploy, execute, check, state, rfc_* |

## Ключевая архитектурная проблема

Действия выполняются по **трём путям**, и только у одного есть личность юзера:

- **API** — Bearer-токен → `*auth.User` в контексте (`middleware.UserContextKey`).
- **CLI** — личности нет, auth не проходит. `cli/deploy.go:58` и др. зовут
  `annetService.ExecuteCommand` напрямую.
- **SSH-сервер** (`internal/ssh/server.go`) — `NoClientAuth: true`, личности нет,
  исполняет действия через тот же `annetService`.

Хук только в API-middleware **пропустил бы весь CLI/SSH-трафик**. Поэтому:

> Запись вешаем на общий **service-choke-point** (`annet.Service.ExecuteCommand`,
> `gnetcli.Client.Exec*`, `check.Devices`), а «кто/откуда» прокидываем через
> `context` — так же, как уже прокинуты `request_id`/`user` в
> `internal/logging/logger.go:15-20,89-100`.

Побочно закрывается разрыв: `middleware.GetUser(ctx)` (`middleware/auth.go:61`)
существует, но **не вызывается никем** — личность аутентифицируется и теряется.

Существующий slog+S3 логгер (`internal/logging`) — archival-формат (файл → gzip → S3),
для запросов из UI непригоден. Аудит — отдельное queryable-хранилище.

---

## 1. Пакет `internal/audit/` (core, без HTTP)

Паттерн фичи из CLAUDE.md (core→REST→CLI→MCP), как `internal/check`.

### `event.go` — модель записи
snake_case JSON-теги, структурный `Error{Type,Message}` с именованными константами
(как требует CLAUDE.md для диагностических фич).

```go
type Event struct {
    ID         int64          `json:"id"`
    Timestamp  time.Time      `json:"timestamp"`
    Actor      string         `json:"actor"`        // user.Name | os-user | "ssh" | "legacy"
    ActorRole  string         `json:"actor_role,omitempty"`
    Source     string         `json:"source"`       // "api" | "cli" | "ssh" | "mcp"
    Action     string         `json:"action"`       // gen|diff|patch|deploy|execute|check|state|rfc_*
    Devices    []string       `json:"devices,omitempty"`
    Command    string         `json:"command,omitempty"`  // сырая команда для execute
    Params     map[string]any `json:"params,omitempty"`   // generators/dry_run/…
    Success    bool           `json:"success"`
    DurationMs int64          `json:"duration_ms,omitempty"`
    Error      *Error         `json:"error,omitempty"`
    RequestID  string         `json:"request_id,omitempty"`
}
```

### `context.go` — ключи и хелперы
Зеркалят `logging` (тип `contextKey`): `ActorKey`, `ActorRoleKey`, `SourceKey`;
`WithActor(ctx, name, role, source)`, `ActorFrom(ctx)`. Константы источников:
`SourceAPI/CLI/SSH/MCP`.

### `recorder.go` — интерфейс + реализации
```go
type Recorder interface {
    Record(ctx context.Context, e Event)                     // best-effort, БЕЗ ошибки
    List(ctx context.Context, f Filter) ([]Event, int, error)
    Close() error
}
```
- **`NopRecorder`** — когда `Audit.Enabled=false` (дефолт). Все хуки зовут `Record`
  безусловно → поведение не ломается, БД не нужна.
- **`PostgresRecorder`** — pgx/v5 + `pgxpool`. **Асинхронная запись**: `Record` кладёт
  событие в буферизованный канал, фоновый воркер батчами пишет в PG. Канал полон / PG
  недоступен → лог через `logging.Error` и **дроп**. Никогда не роняем сетевую операцию.
  `Close()` флашит остаток.

### `store.go` — запросы
`Filter{Actor, Device, Action, Source, From, To, Success *bool, Limit, Offset}`.
`List` — параметризованный SQL (от инъекций), сортировка `timestamp DESC`, возвращает
срез + total. Схема через `go:embed schema.sql`, применяется идемпотентно на старте:
`CREATE TABLE IF NOT EXISTS audit_events (...)` + индексы по `timestamp`, `actor`,
`action`, `source`. `devices` → `text[]`/`jsonb`, `params` → `jsonb`. Мигратор простой
idempotent-create; golang-migrate на старте не тащим.

### `audit_test.go`
Nop-путь без БД; PG-путь опционально за env-гейтом (или testcontainers), чтобы
`make test` не требовал Postgres. Учесть гочу тестов инвентаря, если события ссылаются
на устройства (см. память `annet-oil-feature-pattern`).

## 2. Config — `internal/config/config.go`

Новое опциональное поле в `Config` (паттерн `S3LogConfig`/`JiraConfig`, `omitempty`+`Enabled`):

```go
Audit AuditConfig `yaml:"audit,omitempty"`

type AuditConfig struct {
    Enabled    bool   `yaml:"enabled,omitempty"`
    DSN        string `yaml:"dsn,omitempty"`     // postgres://…  (или поля ниже)
    Host       string `yaml:"host,omitempty"`
    Port       int    `yaml:"port,omitempty"`
    User       string `yaml:"user,omitempty"`
    Password   string `yaml:"password,omitempty"`
    Database   string `yaml:"database,omitempty"`
    SSLMode    string `yaml:"ssl_mode,omitempty"`
    BufferSize int    `yaml:"buffer_size,omitempty"` // размер async-очереди
}
```
Добавить выключенный блок в дефолтный конфиг-шаблон (default-writer в `config.go`).

## 3. Точки записи (покрывают API + CLI + SSH)

Внедряем `audit.Recorder` в исполнительные компоненты через конструкторы — не в
HTTP-слой. Каждый хук читает actor/source из `ctx` через `audit.ActorFrom`.

- **`internal/annet/commands.go`** — поле `recorder audit.Recorder` в `Service`,
  параметр в `New(...)` (`commands.go:50`). В конце `ExecuteCommand` (`commands.go:58`)
  собрать `Event`: Action=`req.Command`, Devices=`req.Filters`, Params (generators/dry_run),
  Success/DurationMs/Error из `CommandResponse`. → gen/diff/patch/deploy для **всех** путей.
- **`internal/gnetcli/client.go`** — recorder в `Client`; в `Exec`/`ExecWithDevice`
  (`client.go:70,103`) писать Action=`execute`, Command=cmd, Devices=[host], результат из
  `ExecResult`. → `/execute` и `cli/state`.
- **`internal/check/`** — recorder в `check.Devices` (`check.go`, зовётся из `cli/check.go:99`),
  Action=`check`. Один хук на оба пути.
- **`internal/api/handlers/rfc.go`** — писать `rfc_create/submit/close/comment` в хендлере
  (RFC — только API/MCP-путь).

## 4. Прокидывание «кто/откуда» в context

- **API**: новый `apimiddleware.AuditContextMiddleware` после `AuthMiddleware`
  (`server.go:49`) — берёт `*auth.User` через `middleware.GetUser(ctx)` и кладёт
  `WithActor(ctx, user.Name, user.Role.Name, SourceAPI)`.
- **CLI**: в `initializeServices()` (`root.go:107`) создать recorder, передать в
  `annet.New`/gnetcli/check. Базовый ctx обогащать `WithActor(ctx, os-user, "", SourceCLI)`
  (`os/user.Current` / `$USER`).
- **SSH**: в `internal/ssh/server.go` при dispatch — `WithActor(ctx, remoteAddr, "", SourceSSH)`.
  Recorder придёт вместе с `annetService`.
- **MCP** ходит через REST → покрывается API-путём (source=`api`).

## 5. Чтение — `/api/v0/audit` (REST + CLI + MCP)

- **REST**: `internal/api/handlers/audit.go` → `NewAuditHandler(rec audit.Recorder)`,
  `GET /api/v0/audit` c query-параметрами `actor,device,action,source,from,to,limit,offset`,
  ответ `{events:[…], total:N}`. Смонтировать в `server.go` (`r.Mount("/audit", …)`).
  Пробросить recorder в `Server` (`server.go:19-37`, `NewServer`).
- **CLI**: `internal/cli/audit.go` — команда `annet-oil audit` (последние события, фильтры),
  регистрация в `init()`.
- **MCP**: `mcp-annet-oil/src/index.ts` — tool `annet_audit` (entry в `tools[]` + `case`
  в switch) + типизированный метод и result-тип в `client.ts`.

## 6. Жизненный цикл / зависимости

- Новая зависимость: `github.com/jackc/pgx/v5` (+ `pgxpool`) — первая БД в проекте (`go.mod`).
- Единая фабрика `audit.NewRecorder(cfg.Audit)` → PG или Nop. Вызвать в CLI-wiring
  (`root.go`), передать во все компоненты; `defer rec.Close()` + флаш при graceful
  shutdown (`cmd/annet-oil`).
- Схема применяется на старте recorder'а (idempotent).

---

## Критические файлы

**Новые:** `internal/audit/{event,context,recorder,store,schema.sql,audit_test}.go`,
`internal/api/handlers/audit.go`, `internal/cli/audit.go`.

**Правки:** `internal/config/config.go`, `internal/annet/commands.go`,
`internal/gnetcli/client.go`, `internal/check/*.go` + `internal/cli/check.go`,
`internal/api/handlers/rfc.go`, `internal/api/middleware/auth.go`,
`internal/api/server.go`, `internal/cli/root.go`, `internal/ssh/server.go`,
`cmd/annet-oil`, `go.mod`.

**MCP:** `mcp-annet-oil/src/index.ts`, `mcp-annet-oil/src/client.ts`.

## Проверка (end-to-end)

1. `make test` — Nop-путь и core-логика без БД (Postgres не требуется).
2. Поднять PG (`docker run … postgres`), в конфиге `audit.enabled: true` + DSN.
3. `make build`. Запустить `annet-oil diff --filters <host>` (CLI) и тот же diff через
   `POST /api/v0/diff` c Bearer-токеном (API).
4. `GET /api/v0/audit` → обе записи есть, с корректными `source` (`cli` vs `api`),
   `actor` (os-user vs имя из токена), `action`/`devices`/`success`.
5. `annet-oil execute <host> "show version"` → запись `action=execute` с командой.
6. Выключенный режим (`audit.enabled: false`) — всё работает, записей нет, БД не нужна.
7. `make lint` / `make check`.

## Расширения на будущее (не в первой версии)

- Multi-instance: с Postgres уже готово к нескольким инстансам annet-oil (конкурентная запись).
- Ретеншн/партиционирование `audit_events` по времени, TTL-очистка.
- Дифф-снапшоты: сохранять сам вывод `diff`/`patch` (не только факт запуска) для показа в UI.