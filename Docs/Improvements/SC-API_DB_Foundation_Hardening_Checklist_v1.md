# SC-API — DB Foundation Hardening Checklist (v1.0)

> Цель: **простой, надёжный, предсказуемый** доступ к БД для `sc-api` с чистой семантикой PostgreSQL, потокобезопасными кэшами, пулами per-DB, корректной готовностью/здоровьем и готовностью к будущему multi-instance (без внедрения прямо сейчас).

---

## 0) Скоуп этой фазы

- **Один** инстанс Postgres (из ENV).  
- Поддерживаем режимы изоляции упрощённо: `shared` и `dedicated_db` (без `schema-per-tenant` в этой фазе).
- В рантайме: **пул per-DB** (ленивый), в мастер-БД — общий пул.
- Удаляем (или делаем no-op) подстановку `{db}` в SQL — **вся маршрутизация БД** идёт через выбор пула, **не** через строку запроса.
- Кэш `tenant → db_name` с инкрементальным update по `(updated_at, id)` и блокировками.

---

## 1) ENV и базовый DSN

### 1.1. ENV (только один инстанс)
```env
SCAPI_DB_HOST=127.0.0.1
SCAPI_DB_PORT=5432
SCAPI_DB_USERNAME=postgres
SCAPI_DB_PASSWORD=postgres
SCAPI_DB_MASTERNAME=sc-master
SCAPI_DB_SSLMODE=disable   # optional; 'disable' by default
# Пулы по-умолчанию (можно оставить дефолты в коде)
SCAPI_DB_POOL_MAX_IDLE=10
SCAPI_DB_POOL_MAX_OPEN=50
SCAPI_DB_POOL_MAX_LIFETIME=30m
SCAPI_DB_POOL_MAX_IDLE_TIME=15m
```

### 1.2. Формирование DSN  
Храните **исходный DSN-стринг как есть**, и при открытии tenant-пула меняйте **только `dbname`**. Не теряйте неизвестные параметры (например, `connect_timeout`, `sslrootcert`).

Простой безопасный способ — хранить базовый DSН как строку плюс аккуратная замена `dbname=...` регекспом *или* формирование через структуру параметров, которая **не отбрасывает** неизвестные ключи (если остаетесь на `pq`). Если перейдёте на `pgx`, используйте парсер URL/Config.

---

## 2) Схемы таблиц (DDL) и миграции

### 2.1. `tenant` (master DB)
```sql
-- 026_tenant_updated_at.sql
ALTER TABLE IF EXISTS tenant
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

-- ensure data is consistent
UPDATE tenant SET updated_at = now() WHERE updated_at IS NULL;

-- optional: composite index to support cursor (updated_at, id)
CREATE INDEX IF NOT EXISTS tenant_updated_at_id_idx
  ON tenant (updated_at, id);

-- базовая структура (для справки)
-- id BIGINT GENERATED ALWAYS AS IDENTITY (START WITH 100 INCREMENT BY 1),
-- name text NOT NULL,
-- subdomain text UNIQUE,
-- is_sandbox boolean NOT NULL DEFAULT false,
-- isolation text NOT NULL DEFAULT 'shared' CHECK (isolation IN ('shared','dedicated_db')),
-- plan text NOT NULL DEFAULT 'light' CHECK (plan IN ('light','pro','enterprise')),
-- db_name text NOT NULL,
-- created_at timestamptz NOT NULL DEFAULT now()
```

### 2.2. (Опционально) `tenant_domain`
```sql
CREATE TABLE IF NOT EXISTS tenant_domain (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  host text NOT NULL,
  is_primary boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(host)
);
CREATE INDEX IF NOT EXISTS tenant_domain_tenant_id_idx ON tenant_domain(tenant_id);
```

---

## 3) Загрузка кэша `tenant → db_name` (инкрементальная, безопасная)

### 3.1. SQL курсор по `(updated_at, id)`
```sql
-- Full snapshot (on cold start):
SELECT CAST(id AS text) AS tenant_id, db_name, updated_at, id
FROM tenant
ORDER BY updated_at, id;

-- Incremental (cursor):
-- держим last_updated_at (ts) и last_id (bigint) в памяти клиента
SELECT CAST(id AS text) AS tenant_id, db_name, updated_at, id
FROM tenant
WHERE (updated_at, id) > ($1::timestamptz, $2::bigint)
ORDER BY updated_at, id;
```

### 3.2. Обновление кэша (Go, потокобезопасно)
```go
// fields in Client:
// dbnames map[string]string   // tenantId -> dbName
// updated time.Time
// lastID int64
// mu sync.RWMutex             // protects dbnames/updated/lastID

// Return copy to avoid external mutation.
func (c *Client) GetTenantDbNamesCopy() map[string]string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    out := make(map[string]string, len(c.dbnames))
    for k, v := range c.dbnames {
        out[k] = v
    }
    return out
}
```

```go
// Incremental refresh with (updated_at, id) cursor and throttling with jitter.
func (c *Client) loadDbNamesLocked(ctx context.Context) (updatedAny bool, err error) {
    // NOTE: Caller must hold c.mu.Lock()

    // Throttle refresh to avoid hammering master DB
    const base = time.Minute * 5
    jitter := time.Duration(rand.Int63n(int64(time.Minute))) // 0..1m
    if !c.updated.IsZero() && time.Since(c.updated) < base-jitter {
        return false, nil
    }

    var rows *sql.Rows
    if c.updated.IsZero() {
        rows, err = c.masterDB.QueryContext(ctx, `
            SELECT CAST(id AS text) AS tenant_id, db_name, updated_at, id
            FROM tenant
            ORDER BY updated_at, id`)
    } else {
        rows, err = c.masterDB.QueryContext(ctx, `
            SELECT CAST(id AS text) AS tenant_id, db_name, updated_at, id
            FROM tenant
            WHERE (updated_at, id) > ($1, $2)
            ORDER BY updated_at, id`, c.updated, c.lastID)
    }
    if err != nil { return false, err }
    defer rows.Close()

    if c.dbnames == nil {
        c.dbnames = make(map[string]string)
    }

    updatedAny = false
    maxTS, maxID := c.updated, c.lastID

    for rows.Next() {
        var (
            tenantID string
            dbName   string
            ts       time.Time
            id       int64
        )
        if err := rows.Scan(&tenantID, &dbName, &ts, &id); err != nil {
            return false, err
        }
        c.dbnames[tenantID] = dbName
        if ts.After(maxTS) || (ts.Equal(maxTS) && id > maxID) {
            maxTS, maxID = ts, id
        }
        updatedAny = true
    }
    if err := rows.Err(); err != nil { return false, err }

    // Advance cursor only if we saw any rows.
    if updatedAny {
        c.updated, c.lastID = maxTS, maxID
    } else if c.updated.IsZero() {
        // Prevent tight loop on empty table on cold start.
        c.updated = time.Now()
        c.lastID = 0
    }
    return updatedAny, nil
}
```

---

## 4) Открытие пулов per-DB (лениво), эвикция, таймауты

### 4.1. Структура пула
```go
type pooledDB struct {
    db       *sql.DB
    lastUsed atomic.Int64 // unix seconds
}

func (p *pooledDB) touch() {
    p.lastUsed.Store(time.Now().Unix())
}
```

### 4.2. Карта пулов и открытие
```go
// in Client:
poolsMu  sync.RWMutex
pools    map[string]*pooledDB
poolCfg  poolConfig
baseDSN  string // keep original DSN string, do not drop unknown keys
```

```go
func (c *Client) getOrOpenDB(dbName string) (*sql.DB, error) {
    name := strings.TrimSpace(dbName)
    if name == "" { return nil, fmt.Errorf("db name required") }

    // Fast path
    c.poolsMu.RLock()
    if pdb, ok := c.pools[name]; ok && pdb != nil && pdb.db != nil {
        c.poolsMu.RUnlock()
        pdb.touch()
        return pdb.db, nil
    }
    c.poolsMu.RUnlock()

    // Slow path
    c.poolsMu.Lock()
    defer c.poolsMu.Unlock()

    if pdb, ok := c.pools[name]; ok && pdb != nil && pdb.db != nil {
        pdb.touch()
        return pdb.db, nil
    }

    // Build DSN with new dbname.
    dsn := replaceDBName(c.baseDSN, name) // implement safe replace of dbname=...
    db, err := sql.Open("postgres", dsn)
    if err != nil { return nil, fmt.Errorf("open db '%s': %w", name, err) }

    // Pool tuning
    db.SetMaxIdleConns(c.poolCfg.MaxIdle)
    db.SetMaxOpenConns(c.poolCfg.MaxOpen)
    db.SetConnMaxLifetime(c.poolCfg.MaxLifetime)
    db.SetConnMaxIdleTime(c.poolCfg.MaxIdleTime)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        _ = db.Close()
        return nil, fmt.Errorf("ping db '%s': %w", name, err)
    }

    pdb := &pooledDB{db: db}
    pdb.touch()
    c.pools[name] = pdb
    return db, nil
}
```

### 4.3. Эвикция неиспользуемых пулов
```go
// Run once at startup.
func (c *Client) startPoolJanitor(stop <-chan struct{}) {
    ttl := 60 * time.Minute  // idle TTL
    tick := time.NewTicker(10 * time.Minute)
    go func() {
        for {
            select {
            case <-tick.C:
                cutoff := time.Now().Add(-ttl).Unix()
                c.poolsMu.Lock()
                for name, pdb := range c.pools {
                    if pdb == nil || pdb.db == nil { delete(c.pools, name); continue }
                    if pdb.lastUsed.Load() < cutoff {
                        _ = pdb.db.Close()
                        delete(c.pools, name)
                    }
                }
                c.poolsMu.Unlock()
            case <-stop:
                tick.Stop()
                return
            }
        }
    }()
}
```

---

## 5) `Database` слой — убрать `{db}` и трейсинг

### 5.1. Удалить подстановку `{db}`
```go
// Before: query = strings.ReplaceAll(query, "{db}", d.Name)
// Now: no-op. The pool already points to the right physical DB.
// If you must keep compatibility for a while, guard it:
func (d *Database) setDbName(query string) string {
    // WARNING: no-op, pools are per-DB now
    return query
}
```

### 5.2. Трейс и логирование (sanitize args)
```go
func (d *Database) traceQuery(err error, dur time.Duration, query string, args ...any) {
    // Mask secrets in args before logging
    safeArgs := sanitizeArgs(args)
    if err != nil {
        d.client.logger.Error("db.query", "ms", dur.Milliseconds(), "err", err, "sql", query, "args", safeArgs)
        return
    }
    if dur > longRunningQueryThreshold {
        d.client.logger.Warn("db.query.slow", "ms", dur.Milliseconds(), "sql", query, "args", safeArgs)
        return
    }
    d.client.logger.Debug("db.query", "ms", dur.Milliseconds(), "sql", query, "args", safeArgs)
}
```

---

## 6) Ретраи (только транзиентные ошибки)
```go
// Simplified list of transient PostgreSQL states
var transient = map[string]struct{}{
    "40001": {}, // serialization_failure
    "40P01": {}, // deadlock_detected
    "53300": {}, // too_many_connections
    "57P01": {}, // admin_shutdown
    "08006": {}, // connection_failure
    "08001": {}, // sqlclient_unable_to_establish_sqlconnection
}

func canRetry(err error) bool {
    var pqe *pq.Error
    if errors.As(err, &pqe) {
        _, ok := transient[string(pqe.Code)]
        return ok
    }
    // Also consider net errors
    var netErr net.Error
    if errors.As(err, &netErr) && netErr.Temporary() {
        return true
    }
    return false
}

func runWithRetry[T any](fn func() (T, error)) (T, error) {
    var zero T
    const maxAttempts = 2
    backoff := time.Duration(50+rand.Intn(50)) * time.Millisecond // small jitter
    for i := 0; i < maxAttempts; i++ {
        out, err := fn()
        if err == nil { return out, nil }
        if i == maxAttempts-1 || !canRetry(err) {
            return zero, err
        }
        time.Sleep(backoff)
        backoff *= 2
    }
    return zero, nil
}
```

---

## 7) Health/Ready

### 7.1. `/healthz` — liveness
- Возвращает **200**, если процесс жив, logger/конфиг инициализированы, masterDB был открывался.  
- **Не** проверяет миграции и tenant-пулы.

```go
func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}
```

### 7.2. `/readyz` — readiness
- Возвращает **503**, пока:
  - `masterDB.PingContext()` не проходит;
  - миграции не завершены (`migrate.ApplyAll...` статус неок);
- После — **200**. В этой фазе **не** требуем пинг каждого tenant-пула (они ленивые).

```go
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    if err := s.sqlClient.Master().PingContext(ctx); err != nil {
        http.Error(w, `{"ready":false,"reason":"master_ping"}`, http.StatusServiceUnavailable)
        return
    }
    if !s.migrationsDone.Load() {
        http.Error(w, `{"ready":false,"reason":"migrations"}`, http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"ready":true}`))
}
```

---

## 8) Метрики (P2, но место заложить сейчас)

- **Counters**: `db_pools_open_total`, `db_pools_evicted_total`, `db_query_errors_total{code}`.  
- **Gauges**: `db_pools_alive`, `db_pool_open_conns{db}`, `db_pool_in_use{db}`, `db_pool_idle{db}`.  
- **Histograms**: `db_query_duration_seconds{db,stmt}` (stmt можно группировать по “name”).  

> На этой фазе достаточно интерфейсов/точек расширения; фактическую интеграцию Prometheus можно отложить.

---

## 9) Тест-план (минимум, но жёстко)

### 9.1. Unit + Race
- `go test ./... -race` — **без гонок**.
- Параллельные `OpenDB(tenantX)` 100 горутин → один пул на `db_name`.  
- Инкрементальные обновления кэша: два ряда с одинаковым `updated_at`, разными `id` — оба считываются.

### 9.2. Integration (docker-postgres / testcontainers)
- Создать master DB + 3 app DB (`sc-app`, `sc-a`, `sc-b`), вставить `tenant` строки.
- Проверить:
  - `ApplyMigrations` обходит уникальные `db_name`.
  - `OpenDB(tenantA)` → запрос попадает в `sc-a` (проверить `SELECT current_database()`).
  - Janitor закрывает простойные пулы.

### 9.3. E2E (httptest)
- `/healthz` сразу 200.
- `/readyz` 503 пока не завершены миграции → после 200.
- Под нагрузкой (vegeta 500 rps короткие SELECT) — нет дубликатов пулов, нет гонок.

---

## 10) Таблица «Что именно делаем»

| Блок | Действие | Файл/Компонент | Готово |
|---|---|---|---|
| DDL | Добавить `updated_at`, индекс `(updated_at,id)` | `migrations/*/026_tenant_updated_at.sql` | ☐ |
| Cache | Ввести `mu` (RWMutex) для `dbnames/updated/lastID` | `internal/sqlserver/client.go` | ☐ |
| Cache | `GetTenantDbNamesCopy()` возвращает копию | `internal/sqlserver/client.go` | ☐ |
| Cache | Курсор `(updated_at,id)` и джиттер-троттлинг | `internal/sqlserver/client.go` | ☐ |
| Pools | `pools map[string]*pooledDB` + `startPoolJanitor` | `internal/sqlserver/client.go` | ☐ |
| Pools | `MaxIdle`, `MaxOpen`, `MaxLifetime`, `MaxIdleTime` | `internal/sqlserver/client.go` | ☐ |
| DSN | Без потери неизвестных ключей (safe replace dbname) | `internal/sqlserver/client.go` | ☐ |
| DB API | Удалить/выключить `{db}` подстановку | `internal/sqlserver/database.go` | ☐ |
| Retry | Реальный `canRetry` + backoff с джиттером | `internal/sqlserver/retry.go` | ☐ |
| Logs | Маскирование аргументов, структ. логи | `internal/sqlserver/database.go` | ☐ |
| Health | `/healthz` и `/readyz` по правилам выше | `cmd/scapi/internal/server/routes.go` | ☐ |
| Tests | Unit+Race, Integration, E2E | `*_test.go` | ☐ |
| Docs | README «Как работает OpenDB и пулы» | `docs/architecture/db-open-db.md` | ☐ |

---

## 11) Частые ошибки и анти-паттерны (избежать)

- ⛔ Подстановка `{db}` для реальных запросов — **убираем** (или no-op).
- ⛔ Возврат внутренней `map` кэша без копии — **только копию**.
- ⛔ Многократное открытие пулов на один `db_name` без double-check под write-lock.
- ⛔ Потеря неизвестных DSN-параметров при разборе — **не терять**.
- ⛔ Логи с секретами/длинным бинарным payload — **маскирование + лимиты**.

---

## 12) Мини-FAQ

**Зачем пулы per-DB, если раньше работало `{db}`?**  
Чистая семантика PG, управляемость, изоляция, мониторинг, предсказуемость — особенно важно для `dedicated_db`.

**Нужен ли `schema-per-tenant`?**  
Хорошо, но это другая фаза. Сейчас — стабилизируем per-DB пулы и кэш.

**А multi-instance Postgres (S1..S5)?**  
Добавим позже таблицу `db_instance` и `tenant_db_binding`. Текущая архитектура к этому готова.
