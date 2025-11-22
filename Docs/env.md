## Конфигурация и `.env` для `sc-api`

Этот документ описывает, как сервис `sc-api` загружает настройки и какие переменные окружения используются в текущей сборке.

### 1. Как загружается конфигурация

- **Загрузка `.env`** (пакет `internal/options`):
  - при запуске можно указать путь к env‑файлу: `--env /path/to/.env`;
  - если `--env` не указан, сервис пытается загрузить:
    - локальный `.env`: `$HOME/.config/scapi/.env` (Unix) или `%USERPROFILE%/scapi/.env` (Windows);
    - глобальный `.env`: `/etc/scapi/.env` (Unix) или рядом с бинарником (Windows).
  - значения из `.env` переопределяют переменные окружения.

- **Конфигурация приложения** (пакет `internal/config`):
  - конфигурация собирается из:
    - встроенных значений (если заданы в `defaultConfiguration`),
    - глобального и локального конфигов (`*.conf`, `*.toml`, `*.yaml`, `*.json`),
    - переменных окружения с префиксами `SCAPI_` и `AWS_`,
    - при необходимости — внешних источников (AWS SSM/Secrets Manager).
  - ключи конфигурации формируются из имён переменных:
    - префикс `SCAPI_` отбрасывается,
    - `_` заменяется на `.`,
    - имя приводится к нижнему регистру.
  - пример: `SCAPI_HOSTAPP` → ключ `hostapp`, `SCAPI_DB_HOST` → `host`.

### 2. Основные переменные окружения

Ниже приведён рекомендованный набор переменных окружения для работы `sc-api`. Префикс по умолчанию — `SCAPI_`.

- **2.1. Настройки HTTP‑сервера**
  - **`SCAPI_HOST`**
    - Назначение: адрес и порт для прослушивания HTTP‑сервера.
    - Тип: строка (`[address]:port`), например `:8080` или `0.0.0.0:8080`.
    - Обязательность: нет (может быть задано через конфиг или флаг `--host`).
    - Пример: `SCAPI_HOST=:8080`.

  - **`SCAPI_HOSTAPP`**
    - Назначение: значение поля `hostapp` в конфигурации, используется как `Addr` для `http.Server`.
    - Тип: строка.
    - Обязательность: да, если не задано через файл конфигурации.
    - Пример: `SCAPI_HOSTAPP=:8080`.

- **2.2. CORS и фронтенд‑origin**
  - **`SCAPI_ORIGIN`**
    - Назначение: допустимый Origin фронтенд‑клиента, используется в CORS‑middleware.
    - Тип: строка (полный Origin или маска вида `https://app.example.com`).
    - Обязательность: да, для корректной работы CORS.
    - Пример: `SCAPI_ORIGIN=https://app.safeconstructors.com`.

- **2.2.1. Парсинг токена**
  - **`SCAPI_TOKEN_PROVIDER`**
    - Назначение: определяет, как парсятся JWT-клеймы в middleware (`internal` или `cognito`).
    - Тип: строка.
    - Обязательность: нет (по умолчанию `internal`).
    - Поведение:
      - `internal` — ожидается, что токен содержит `tenant_id`, `user_id`, `email`.
      - `cognito` — используется Cognito JWT (поля `custom:tenant_id`, `custom:user_id`, `email`); при отсутствии кастомных полей fallback на `username`.
    - Пример: `SCAPI_TOKEN_PROVIDER=cognito`.

- **2.3. Настройки подключения к PostgreSQL**

Конфигурация БД маппится на поля структуры `server.Config` (`json:"host"`, `json:"port"`, `json:"username"`, `json:"password"`, `json:"dbname"` и др.), которые заполняются через конфиг/ENV. На их основе формируется строка подключения:

```text
host=<DbHost> port=<DbPort> dbname=<DbName> user=<DbUser> password=<DbPass> sslmode=disable
```

Рекомендуемые переменные:

  - **`SCAPI_DB_HOST`**
    - Назначение: хост PostgreSQL.
    - Тип: строка.
    - Обязательность: да.
    - Пример: `SCAPI_DB_HOST=postgres`.

  - **`SCAPI_DB_PORT`**
    - Назначение: порт PostgreSQL.
    - Тип: строка (будет приведён к числу драйвером).
    - Обязательность: да.
    - Пример: `SCAPI_DB_PORT=5432`.

  - **`SCAPI_DB_USERNAME`**
    - Назначение: пользователь PostgreSQL.
    - Тип: строка.
    - Обязательность: да.
    - Пример: `SCAPI_DB_USERNAME=scapi_user`.

  - **`SCAPI_DB_PASSWORD`**
    - Назначение: пароль пользователя PostgreSQL.
    - Тип: строка (секрет).
    - Обязательность: да.
    - Пример: `SCAPI_DB_PASSWORD=change_me`.

  - **`SCAPI_DB_DBNAME`**
    - Назначение: имя базы данных PostgreSQL, к которой подключается пул (используется для чтения таблицы `client_config` и, при необходимости, общих структур).
    - Тип: строка.
    - Обязательность: да.
    - Пример: `SCAPI_DB_DBNAME=scapi_master`.

- **2.3.1. Настройки пула подключений к БД**

  Эти параметры позволяют управлять размером и временем жизни пулов подключений `sql.DB` как для master‑БД, так и для tenant‑БД. Все они опциональны; при отсутствии используются значения по умолчанию.

  - **`SCAPI_DB_POOL_MAXIDLE`**
    - Назначение: максимальное число простаивающих (idle) соединений в пуле.
    - Тип: целое число (`int`).
    - Обязательность: нет.
    - Значение по умолчанию: `10`.
    - Пример: `SCAPI_DB_POOL_MAXIDLE=10`.

  - **`SCAPI_DB_POOL_MAXOPEN`**
    - Назначение: максимальное число открытых соединений в пуле.
    - Тип: целое число (`int`).
    - Обязательность: нет.
    - Значение по умолчанию: `50`.
    - Пример: `SCAPI_DB_POOL_MAXOPEN=50`.

  - **`SCAPI_DB_POOL_MAXLIFETIME`**
    - Назначение: максимальное время жизни одного соединения в пуле.
    - Тип: строка‑длительность в формате `time.Duration` (например, `30m`, `1h`).
    - Обязательность: нет.
    - Значение по умолчанию: `30m`.
    - Пример: `SCAPI_DB_POOL_MAXLIFETIME=30m`.

  - **`SCAPI_DB_POOL_MAXIDLETIME`**
    - Назначение: максимальное время простоя (idle) одного соединения в пуле перед его закрытием.
    - Тип: строка‑длительность в формате `time.Duration` (например, `5m`, `10m`).
    - Обязательность: нет.
    - Значение по умолчанию: без ограничения (используется значение по умолчанию драйвера).
    - Рекомендованное значение для продакшена: `10m`.
    - Пример: `SCAPI_DB_POOL_MAXIDLETIME=10m`.

- **2.4. Тестовые переменные**
  - **`SCAPI_TEST_PG_CONN`**
    - Назначение: строка подключения к тестовой базе PostgreSQL для интеграционных тестов `internal/sqlserver`.
    - Тип: строка (DSN формата `host=... port=... dbname=... user=... password=... sslmode=disable` или `postgres://...`).
    - Обязательность: нет, если не запускаются интеграционные тесты.
    - Пример: `SCAPI_TEST_PG_CONN=host=localhost port=5432 dbname=scapi_test user=scapi_test password=secret sslmode=disable`.

### 3. Пример `.env` файла

Ниже приведён пример файла `.env`, подходящий для локальной разработки:

```env
# HTTP server
SCAPI_HOSTAPP=:8080
SCAPI_ORIGIN=http://localhost:3000

# PostgreSQL connection
SCAPI_DB_HOST=localhost
SCAPI_DB_PORT=5432
SCAPI_DB_USERNAME=scapi
SCAPI_DB_PASSWORD=changeme
SCAPI_DB_DBNAME=scapi_master

# Optional DB pool settings
# SCAPI_DB_POOL_MAXIDLE=10
# SCAPI_DB_POOL_MAXOPEN=50
# SCAPI_DB_POOL_MAXLIFETIME=30m
# SCAPI_DB_POOL_MAXIDLETIME=10m


#SCAPI_TOKEN_PROVIDER=cognito

# (опционально) тестовая БД для интеграционных тестов
# SCAPI_TEST_PG_CONN=host=localhost port=5432 dbname=scapi_test user=scapi_test password=secret sslmode=disable
```

### 4. Дополнительно

- **AWS‑интеграции**  
  Если конфигурация (включая параметры подключения к БД) хранится в AWS SSM или AWS Secrets Manager, их можно указать через специальный `config`‑source (см. комментарии в `internal/config/config.go`). В этом случае `.env` может содержать только параметры доступа к AWS и ссылку на нужный секрет.

- **Переопределение через флаги**  
  Любую опцию, зарегистрированную через `internal/options.Options`, можно задавать как флаг командной строки. Приоритет: CLI → `.env`/ENV → конфиги → значения по умолчанию.


