# safe-ms-go

Go-фреймворк (`safeproc`) для безопасной обработки данных в микросервисах.

## Что внутри

- строгий JSON decode с лимитом payload
- санитизация/валидация DTO
- безопасные HTTP-ошибки
- middleware: `recover`, `request-id`, `security headers`, `CORS`, `HMAC`, `idempotency`, `rate limit`
- observability middleware: access log + OTel tracing

## Подключение в сервис

```go
import "github.com/nast1g/safe-ms-go/safeproc"
```

## Тесты

```bash
go test ./...
```
