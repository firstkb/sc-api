# Lambda entry point for `sc-api`

## Архитектура
- Обработчик `cmd/scapi/lambda` использует тот же `server.Bootstrap`, что и бинарник `scapi`, поэтому поднимает все слои (PostgreSQL, миграции, роутер, middleware), но не слушает порт.
- `awslabs/aws-lambda-go-api-proxy/httpadapter` преобразует события API Gateway HTTP API (или REST API) в `http.Request` и возвращает стандартный ответ маршрутов `router.Classifier`.
- Инициализация происходит лениво и кэшируется между инвокациями; повторная попытка выполняется автоматически, если предыдущий старт завершился ошибкой.

## Конфигурация и секреты
`options.NewOptions` и `config.GetConfig` продолжают работать по тем же правилам, поэтому Lambda читает настройки из:
1. AWS Systems Manager Parameter Store / Secrets Manager, если указать источник через `--config` или переменную `SCAPI_CONFIG` (см. [`config/config_options.go`](../src/internal/config/config_options.go)).
2. Переменных окружения с префиксом `SCAPI_` (сбрасывается в `options.NewOptions`). Критичные ключи:
   - `SCAPI_DB_HOST`
   - `SCAPI_DB_PORT`
   - `SCAPI_DB_USERNAME`
   - `SCAPI_DB_PASSWORD`
   - `SCAPI_DB_MASTERNAME`
   - `SCAPI_DB_POOL_MAXIDLE`, `SCAPI_DB_POOL_MAXOPEN`, `SCAPI_DB_POOL_MAXLIFETIME`, `SCAPI_DB_POOL_MAXIDLETIME`
   - `SCAPI_ORIGIN`, `SCAPI_TOKEN_PROVIDER`, `SCAPI_TIMEOUT`
3. `AWS_*` префикс остаётся доступным для SDK (например, `AWS_REGION`).

Секреты рекомендуется хранить в AWS Secrets Manager / Parameter Store, предоставив функции роль с правами `ssm:GetParameter`, `secretsmanager:GetSecretValue`.

## Сборка и упаковка
```bash
cd src
GOOS=linux GOARCH=arm64 go build -o ../bin/scapi-lambda ./cmd/scapi/lambda
zip ../bin/scapi-lambda.zip ../bin/scapi-lambda
```
- Для Graviton функций задайте `GOARCH=arm64`, для x86 — `amd64`.
- Убедитесь, что значение `Build` передаётся через `-ldflags "-X 'main.Build=build-id'"`.

## Пример SAM/CloudFormation
```yaml
Resources:
  ScApiFunction:
    Type: AWS::Serverless::Function
    Properties:
      Handler: bootstrap
      Runtime: provided.al2023
      CodeUri: bin/scapi-lambda.zip
      Timeout: 30
      MemorySize: 512
      Architectures: [arm64]
      Environment:
        Variables:
          SCAPI_DB_HOST: ...
          SCAPI_DB_PORT: ...
      Policies:
        - AWSLambdaVPCAccessExecutionRole
        - SSMParameterReadPolicy:
            ParameterName: "/scapi/*"
  ScApiHttp:
    Type: AWS::Serverless::HttpApi
    Properties:
      StageName: prod
      Target: !Sub arn:aws:lambda:${AWS::Region}:${AWS::AccountId}:function:${ScApiFunction}
```

## Локальное тестирование
1. Экспортируйте конфиг (например, `.env`): `export SCAPI_DB_HOST=localhost ...`.
2. Запустите `sam local start-api --env-vars env.json` или используйте `curl` с `sam local invoke --event events/http.json`.
3. Для быстрой проверки можно вызвать handler напрямую:
   ```bash
   go test ./cmd/scapi/lambda -run TestHandleRequestSuccess -v
   ```

## Наблюдаемость
- Логирование остаётся через `slog` и выводится в CloudWatch Logs.
- Классификатор маршрутов `mw.Classifier` добавляет RouteID/Tier, поэтому интеграция с X-Ray/OTel возможна через middleware уровня сервисов.

## Ограничения
- Lambda по-прежнему выполняет миграции при холодном старте. Для больших схем целесообразно отключить их через конфигурацию/флаг.
- Таймаут запроса управляется `SCAPI_TIMEOUT` и оборачивается middleware `Timeout`.
