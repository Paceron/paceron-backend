## Tasks

### Task 1: DTO, sentinel y filtro en Stamp

- [ ] 1.1 `StampRequest` suma `ExcludeDates []string` json `exclude_dates` (`calendar_day_request.go`), con comentario corto (opcional, YYYY-MM-DD, fechas a saltar por completo).
- [ ] 1.2 Nuevo sentinel `ErrCalendarInvalidDate` en `calendar_service.go` + mapeo a `422` en `mapCalendarError` (`calendar_controller.go`).
- [ ] 1.3 En `Stamp`: parsear `exclude_dates` (cualquier elemento no-`YYYY-MM-DD` → `ErrCalendarInvalidDate`, antes de cualquier escritura, tanto en path mock como tx); construir el subconjunto filtrado de `(planDay, fecha)` y hacer que pre-validación, `rows`, guards de cerradas/conflictos, loop de escritura/instanciación, `deleteSupersededInstance` y respuesta operen solo sobre el subconjunto. Set vacío/ausente = camino actual exacto.
- [ ] 1.4 `go build ./...` + `go vet` sobre los paquetes tocados.

### Task 2: Swagger

- [ ] 2.1 Anotación `@Param body` del stamp: documentar `exclude_dates` opcional con ejemplo y su semántica (no cuenta para 409/422, respuesta lo omite); `@Failure 422` suma fecha inválida en `exclude_dates`.
- [ ] 2.2 `swag init -g cmd/api/main.go -d . -o cmd/api/docs --parseDependency --parseInternal` y verificar los 3 artefactos.

### Task 3: Tests

- [ ] 3.1 Real-Postgres: stamp con `exclude_dates` sobre días ocupados conserva fila+instancia intactas (mismo `session_instance_id`, conteo de filas de instancia invariable) y la respuesta excluye esas fechas.
- [ ] 3.2 Guards ignorados: día cerrado y día ocupado en `exclude_dates` con `force=false` → stamp procede (ni 422 ni 409).
- [ ] 3.3 `exclude_dates` con formato inválido → `ErrCalendarInvalidDate`/422 sin escribir nada; fecha fuera de rango → se ignora (stampa todo).
- [ ] 3.4 Rango totalmente excluido → respuesta `[]`, nada escrito.
- [ ] 3.5 Regresión: sin el campo / con `[]`, comportamiento idéntico (pisar con `force`, 409 sin `force`, 422 cerradas — cubrir con los tests existentes de stamp + assert de que `exclude_dates: []` da el mismo resultado).

### Task 4: Docs y verificación final

- [ ] 4.1 `docs/CATALOGO_Y_CALENDARIO.md` §8: semántica de `exclude_dates` en stamp.
- [ ] 4.2 `docs/FRONTEND_IMPACTO_INSTANCIACION.md`: nota aditiva cerrando el Gap 8 (los clientes viejos no requieren acción).
- [ ] 4.3 `openspec validate stamp-exclude-dates --strict` verde.
- [ ] 4.4 `gofmt` solo archivos tocados; `go build ./...`; `go vet ./...`; suite completa con Postgres real (`docker start paceron-test-db`, env `TEST_DB_*`); `make coverage-with-db` ≥ 80 sin tocar el umbral (si sale un número raro, `go clean -cache` y re-correr — bug de cache conocido).
