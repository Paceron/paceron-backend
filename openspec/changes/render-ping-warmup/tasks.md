# Tasks

## 1. Configuración del warmup

- [x] 1.1 Agregar en `cmd/api/config/` la lectura tipada de `WARMUP_BASE_URL` (default `https://paceron-backend-as9c.onrender.com`) y `WARMUP_INTERVAL_MINUTES` (default 13); validar origen HTTP(S) y enteros positivos, y cubrir defaults, overrides y valores inválidos con tests de config.

## 2. Worker HTTP periódico

- [x] 2.1 Crear el worker en `cmd/api/infrastructure/warmup/`: GET inicial asíncrono, ticker configurable, timeout acotado, respuesta no-2xx/error aislado y cancelación por contexto; probar con `httptest` que llama a `/ping`, repite, no bloquea y continúa tras fallas.

## 3. Bootstrap y configuración de Render

- [x] 3.1 Iniciar el worker desde `app.StartApp` —no desde `NewApplication`, que se usa en tests— y declarar `WARMUP_BASE_URL`/`WARMUP_INTERVAL_MINUTES` para los dos servicios en `render.yaml`; verificar el wiring con `go test ./cmd/api/app ./cmd/api/infrastructure/warmup` y revisar que el YAML conserve ambos servicios.

## 4. Documentación y verificación

- [x] 4.1 Documentar las variables, su default, el override por servicio y la limitación de Render Free en `docs/ENVIRONMENTS.md`; verificar que los nombres y ejemplos coincidan con código y `render.yaml`.
- [x] 4.2 Correr `go test ./...`, `go vet ./...` y `openspec validate --strict render-ping-warmup`; revisar que el warmup no agregue rutas ni cambie `GET /ping`.
