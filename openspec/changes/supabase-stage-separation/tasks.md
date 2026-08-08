## 1. Backend

- [x] 1.1 `config.go`: `IsProductionStage()` (escaneo manual de `os.Args`, sin `flag.Parse()` por el orden de `init()`)
- [x] 1.2 `config.go`: `stagedDatabaseURL()` — elige `SUPABASE_PRODUCTION_DATABASE_URL`/`SUPABASE_TESTING_DATABASE_URL`, reemplaza el `DATABASE_URL` genérico
- [x] 1.3 `app/router.go`: log del stage resuelto al arrancar

## 2. Tests

- [x] 2.1 `config_test.go`: `IsProductionStage` (default false, con flag, con flags no relacionados), `stagedDatabaseURL` (default testing, production con flag)
- [x] 2.2 Tests existentes de `loadDBConfig` migrados a los nuevos nombres de env var
- [x] 2.3 `go build`/`go vet`/`go test ./...` verdes

## 3. Deploy

- [x] 3.1 `render.yaml`: dos services (`paceron-backend` → master + `--stage=production`, `paceron-backend-develop` → develop sin flag), branch corregida (`main` → `master`, quirk resuelto)
- [x] 3.2 `.env` local migrado a `SUPABASE_TESTING_DATABASE_URL`/`SUPABASE_PRODUCTION_DATABASE_URL`

## 4. Docs

- [x] 4.1 `docs/ENVIRONMENTS.md` nuevo — mecanismo + checklist manual de Render
- [x] 4.2 `CLAUDE.md`: sección nueva, quirk de `render.yaml`/`branch: main` removido (ya no aplica)
- [x] 4.3 `render.yaml`: declarar todas las keys secretas necesarias (`JWT_SECRET`/`SMTP_HOST`/`GMAIL_USER`/`GMAIL_APP_PASSWORD`) con `sync: false`, para que Render las pida al sincronizar
- [x] 4.4 `render.yaml`: alinear `buildCommand`/`startCommand` (`-o app`/`./app`) con la config real que el usuario armó a mano en Render (sin Blueprint existente)

## 5. Verificación

- [x] 5.1 `go run cmd/api/main.go` sin flag → log confirma `stage=testing`, conecta OK contra el proyecto de testing real
- [x] 5.2 No se probó `--stage=production` contra la DB de producción real (innecesario y riesgoso para un smoke test — la lógica ya está cubierta por tests unitarios)

## 6. Bug encontrado al verificar el deploy de staging (addendum)

- [x] 6.1 `swagger_handler.go`: `serveSwaggerJSON` tenía un switch `?env=local|production` que hardcodeaba `paceron-backend.onrender.com` como "producción" — staging (`paceron-backend-as9c.onrender.com`) caía en ese mismo bucket y mostraba la URL base incorrecta. Reemplazado por `spec["host"] = c.Request.Host`, refleja el dominio real del request, sin lista fija de entornos.
- [x] 6.2 `swagger_custom.html`: mismo problema en el botón "Export as cURL" (mapa `hosts` hardcodeado + detección `localhost` vs todo-lo-demás). Reemplazado por `window.location.origin`.
- [x] 6.3 Verificado local: `curl .../swagger/doc.json -H "Host: paceron-backend-as9c.onrender.com"` → `host` en la respuesta refleja ese valor, no el hardcodeado
