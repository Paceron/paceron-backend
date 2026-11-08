## Tasks

1. Config: agregar `SEEDING_ALLOW_REMOTE` env var y helper de guardia remota en `cmd/api/config/seeding.go`.
2. Seeder main: parsear `--target`, validar guardas (env vars + production block), requerir scope/ventana en remoto, confirmación interactiva + `--yes`.
3. Seeder seed.go: idempotencia estricta en remoto (abortar si ya existen días del seed).
4. Tests: cubrir guardas y modo remoto en `seeding_test.go` y tests del seeder.
5. Documentación: actualizar `RUNBOOK.md` del seeder con modo remoto y guardas.
6. Verificación: `go test ./...` verde; dry-run contra local; prueba real en local con `--target=local`.