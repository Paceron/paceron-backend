# Tasks

## 1. Escenarios reales en calendar_service (Postgres real / mocks existentes)

- [ ] 1.1 Test: asignar con sesión de catálogo borrada (soft-delete) → 422 `ErrCalendarSessionNotFound`; asignar con link de `session_exercises` sin instancia de ejercicio alcanzable → error correspondiente (crear test file `calendar_service_cobertura_test.go`, verificar `go test ./cmd/api/services` verde)
- [ ] 1.2 Tests de variantes `validateDayFields` (rest/other/training con y sin session_id/instancia previa, presencial sin horarios, horario from>=to) + banner `next_training`/`next_cancelled` con instancia nil → `session_name`/`presencial_*` null, y grupo sin nombre → nombre vacío (verificar suite verde)

## 2. Paths mock y conflictos de stamp

- [ ] 2.1 Cubrir ramas mock `s.db == nil` restantes de calendar_service (stamp con plan vacío, Shift con listas invertidas, "no hay DB disponible" en paths no cubiertos) (verificar suite verde con esos tests aislados)
- [ ] 2.2 Cubrir conflictos de stamp no alcanzados (409 con fechas, stamp totalmente excluido, mezcla conflict+cerrado) con Postgres real (verificar suite verde)

## 3. calendar_controller

- [ ] 3.1 Tests de handlers restantes con `mockCalendarService` (~20 missing: ramas de status/validación no cubiertas) (verificar `go test ./cmd/api/controllers` verde)

## 4. session_service y exercise_service

- [ ] 4.1 Cubrir ramas no testeadas de `session_service.go` (~37) con escenarios reales o mock del paquete (verificar suite verde)
- [ ] 4.2 Cubrir ramas no testeadas de `exercise_service.go` (~25) (verificar suite verde)

## 5. Helper FailingDB + ramas de error de DB

- [ ] 5.1 Extraer helper `FailingDB` en `cmd/api/testutils` (handle gorm con condición por operación, sin acoplarse a mensajes) con su propio test mínimo (verificar `go test ./cmd/api/testutils` verde)
- [ ] 5.2 Cubrir envolturas `fmt.Errorf("...: %w", err)` y ramas `customlogger.Error` de `calendar_service.go` y `session_service.go`/`exercise_service.go` usando FailingDB (verificar suite verde)
- [ ] 5.3 Cubrir branches restantes de DAOs calendario/instancias/grupos (~31) alcanzables con Postgres real (verificar `go test ./cmd/api/daos` verde)

## 6. Deuda registrada y verificación final

- [ ] 6.1 Anotar en `docs/DEUDA_TECNICA_Y_PENDIENTES.md`: etapa 2 de coverage (legacy → 90%) como pendiente y trabajo de repuesto cuando no haya gaps/trabajos nuevos (verificar que el doc refleja ambas notas)
- [ ] 6.2 Verificación final: `go clean -cache` + `make coverage-with-db` + analyzer (go-test-coverage) ≥ 84% total, gate 80 PASS, `.testcoverage.yml` intacto; `openspec validate cobertura-calendario-recientes --strict` ok; `gofmt` limpio en archivos del change; suite completa `go test ./...` con Postgres real 0 FAIL; go build/vet limpios (verificar salidas registradas en el reporte)
