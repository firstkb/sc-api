# Руководство по ручному тестированию авторизации

Документ описывает минимальный набор шагов, чтобы локально проверить `authsvc`: OTP (email/SMS), выдачу access/refresh токенов, обновление и logout. Все примеры приведены для стенда `go run ./cmd/scapi`, но одинаково применимы для docker/CI окружений.

---

## 1. Предварительные условия
- Применены миграции master-БД, содержащие таблицы `identity_subject`, `identity_tenant_membership`, `auth_otp`, `auth_refresh_token`.
- В конфигурации заполнены поля `auth.jwt_private_pem_path`, `auth.jwt_public_pem_path`, `auth.audience` и `identity.hmackey`.
- В таблицах `tenant`, `tenant_db`, `tenant_domain` уже присутствует тестовый tenant (далее `TENANT_ID`, `TENANT_DB`, `TENANT_DOMAIN`).
- В tenant-БД создана таблица `users` (из миграций приложения).

---

## 2. Подготовка тестовых данных

### 2.1 Пользователь в tenant-БД
Подключитесь к tenant-БД (`psql TENANT_DB`). Создайте пользователя, который будет проходить OTP-поток:

```sql
INSERT INTO users (id, tenant_id, email, phone, active, created_at)
VALUES ('8a5f9ce7-36db-4f52-b86c-1ad3c30d7de5', TENANT_ID, 'qa.user@example.com', '+15551112233', TRUE, now())
ON CONFLICT (id) DO UPDATE
  SET active = TRUE, email = EXCLUDED.email, phone = EXCLUDED.phone;
```

`id` (UUID) из этой записи далее используется как `tenant_user_id`.

### 2.2 Создание `identity_subject`
1. Убедитесь, что в master-БД включён `pgcrypto`: `CREATE EXTENSION IF NOT EXISTS pgcrypto;`
2. Возьмите значение `identity.hmackey` из конфигурации (оно base64). Выполните:

```sql
WITH key_material AS (
  SELECT decode('BASE64_KEY_HERE', 'base64') AS hmac_key
)
INSERT INTO identity_subject (type, hmac, hmac_kid, is_primary, status)
SELECT
  'email',
  hmac(lower('qa.user@example.com')::bytea, hmac_key, 'sha256'),
  1,  -- HMACKID
  TRUE,
  'active'
FROM key_material
RETURNING id;
```

Сохраните `identity_subject.id` → далее `IDENTITY_ID`.

### 2.3 Привязка в `identity_tenant_membership`

```sql
INSERT INTO identity_tenant_membership
  (tenant_id, identity_id, tenant_user_id, role, level, status)
VALUES
  (TENANT_ID, IDENTITY_ID, '8a5f9ce7-36db-4f52-b86c-1ad3c30d7de5', 'admin', 50, 'active')
ON CONFLICT (tenant_id, identity_id, tenant_user_id) DO UPDATE
  SET role = EXCLUDED.role,
      level = EXCLUDED.level,
      status = EXCLUDED.status,
      updated_at = now();
```

### 2.4 Генерация известного OTP-кода (опционально)
По умолчанию код хранится в виде Argon2-хэша и неизвестен. Чтобы тестировать «под контролем», можно заранее подготовить хэш для любого кода (например, `654321`):

```bash
cat <<'EOF' >/tmp/hash_otp.go
package main

import (
	"fmt"
	"os"

	"github.com/firstkb/sc-api/internal/auth"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: go run hash_otp.go <code>")
	}
	hash, err := auth.HashOTP(os.Args[1])
	if err != nil {
		panic(err)
	}
	fmt.Println(hash)
}
EOF

cd /Volumes/HD/DigitalTriton/firstkb/sc-api/src && go run /tmp/hash_otp.go 654321
```

Полученный `salt:hash` можно вручную прописать в нужную запись `auth_otp`, чтобы знать какой код ввести при верификации.

---

## 3. Запуск сервиса

Пример минимального `.env`:

```env
SCAPI_HOSTAPP=:8080
SCAPI_ORIGIN=https://qa.safeconstructors.local

SCAPI_DB_HOST=localhost
SCAPI_DB_PORT=5432
SCAPI_DB_USERNAME=scapi
SCAPI_DB_PASSWORD=changeme
SCAPI_DB_DBNAME=scapi_master

SCAPI_AUTH_PROVIDER=internal
SCAPI_AUTH_ACCESS_TTL=15m
SCAPI_AUTH_REFRESH_TTL=720h
SCAPI_AUTH_JWT_ALG=rs256
SCAPI_AUTH_JWT_PRIVATE_PEM_PATH=./secrets/private.pem
SCAPI_AUTH_JWT_PUBLIC_PEM_PATH=./secrets/public.pem
SCAPI_AUTH_JWKS_CACHE_TTL=5m
SCAPI_AUTH_OTP_TTL=10m
SCAPI_AUTH_OTP_LENGTH=6
SCAPI_AUTH_RATE_LIMIT_IP=30/m
SCAPI_AUTH_RATE_LIMIT_ADDR=5/5m

SCAPI_IDENTITY_HMACKEY=BASE64_KEY
SCAPI_IDENTITY_HMACKID=1
```

Запуск: `cd src && go run ./cmd/scapi`.

---

## 4. Тестовые сценарии

### 4.1 Запрос OTP
```bash
curl -i \
  -X POST http://localhost:8080/auth/otp/request \
  -H "Content-Type: application/json" \
  -H "Origin: https://qa.safeconstructors.local" \
  -H "X-Forwarded-For: 203.0.113.10" \
  -d '{"email":"qa.user@example.com"}'
```
Ожидаем `200 OK` и запись в `auth_otp`. Если используется известный хэш (раздел 2.4), обновите строку:
```sql
UPDATE auth_otp
SET code_hash = 'SALT:HASH'
WHERE tenant_id = TENANT_ID AND address = 'qa.user@example.com'
ORDER BY created_at DESC LIMIT 1;
```

### 4.2 Верификация OTP + получение токенов

```bash
curl -i \
  -X POST http://localhost:8080/auth/otp/verify \
  -H "Content-Type: application/json" \
  -H "Origin: https://qa.safeconstructors.local" \
  -d '{"email":"qa.user@example.com","code":"654321"}'
```

Успешный ответ содержит `access_token`, `refresh_token`, `expires_in`. Проверьте:
- `auth_refresh_token` в master-БД пополнилась записью.
- `event_log` tenant-БД получил события `otp_verify`, `login`.

### 4.3 Обновление токена

```bash
curl -i \
  -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -H "Origin: https://qa.safeconstructors.local" \
  -d '{"refresh_token":"<значение из шага 4.2>"}'
```

Проверьте, что старый refresh помечен `revoked_at`, а новый добавлен.

### 4.4 Logout

```bash
curl -i \
  -X POST http://localhost:8080/auth/logout \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <access_token>" \
  -H "Origin: https://qa.safeconstructors.local" \
  -d '{"all_devices":true}'
```

После logout все активные refresh-токены пользователя в master-БД должны получить `revoked_at`.

### 4.5 Негативные проверки
- Неверный код (`code="000000"`) → `403 invalid OTP`.
- Отсутствие `Origin`, не зарегистрированного домена или отсутствия tenant-контекста → `403 tenant guard`.
- Повторный `refresh` с уже отозванным токеном → `401 unauthorized`.

---

## 5. Очистка данных

```sql
-- master DB
DELETE FROM auth_refresh_token WHERE tenant_id = TENANT_ID;
DELETE FROM auth_otp WHERE tenant_id = TENANT_ID;
DELETE FROM identity_tenant_membership WHERE tenant_id = TENANT_ID AND tenant_user_id = '8a5f9ce7-36db-4f52-b86c-1ad3c30d7de5';
DELETE FROM identity_subject WHERE id = IDENTITY_ID;

-- tenant DB
DELETE FROM event_log WHERE tenant_id = TENANT_ID;
DELETE FROM users WHERE id = '8a5f9ce7-36db-4f52-b86c-1ad3c30d7de5';
```

После очистки можно повторять сценарий с новыми значениями.

