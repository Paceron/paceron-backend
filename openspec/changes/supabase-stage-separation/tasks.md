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

## 5. Verificación

- [x] 5.1 `go run cmd/api/main.go` sin flag → log confirma `stage=testing`, conecta OK contra el proyecto de testing real
- [x] 5.2 No se probó `--stage=production` contra la DB de producción real (innecesario y riesgoso para un smoke test — la lógica ya está cubierta por tests unitarios)
