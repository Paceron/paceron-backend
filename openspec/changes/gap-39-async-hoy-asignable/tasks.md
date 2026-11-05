## Global Constraints

- Rama `feature/gap-39-async-hoy` (ya creada desde `develop` `eeebc37`).
- Postgres real descartable: `docker start paceron-test-db` y `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`. No tocar base local/demo ni correr `local-*`, `demo-*` o `fullseed-*`.
- Coverage gate 85: `go clean -cache` + `make coverage-with-db` + analyzer; no bajar `.testcoverage.yml`.
- Comentarios mínimos; stagear solo rutas explícitas; no `git add -A`; no commitear `.superpowers/`.
- Convencional Commits; no push ni merge (los hace el usuario).
- Swagger no requiere regeneración (sin cambios de DTOs).

### Task 1: Guard de día cerrado async de hoy

**Files:**
- Modify: `cmd/api/services/calendar_service.go`
- Test: `cmd/api/services/calendar_service_seeding_test.go`, `cmd/api/services/calendar_service_task3_test.go`

- [ ] **1.1** Escribir primero el test que fija el nuevo contrato: `isCalendarDayClosed` con `date == hoy && !is_presencial` debe ser `false`; presencial de hoy con horario pasado sigue `true`; pasado `true` sin seeding; futuro `false`. Correr y verificar RED.
- [ ] **1.2** Cambiar la rama async de hoy en `isCalendarDayClosed` a `false` y actualizar el comentario de la función (el async de hoy ya no cierra).
- [ ] **1.3** Ajustar los dos asserts existentes que esperaban `true` y correr los tests unitarios focales; verificar GREEN.
- [ ] **1.4** `gofmt`, `go build ./...` y `go vet ./...`; commit `feat(calendar): permitir escritura en el dia async de hoy (Gap 39)`.

### Task 2: Regresiones de servicio y tests afectados

**Files:**
- Test: `cmd/api/services/calendar_service_cobertura_test.go`, `cmd/api/services/calendar_service_exclude_test.go`, `cmd/api/services/calendar_service_gap39_test.go` (nuevo)

- [ ] **2.1** Agregar regresión con Postgres real: `UpsertDay` de hoy async con `kind=training` crea el día y su instancia; presencial de hoy ya iniciada sigue devolviendo `422 ErrCalendarDayClosed`.
- [ ] **2.2** Rediseñar `TestCobertura_StampConflictMezclaCerradoGana` con ayer como día cerrado y hoy como conflicto abierto, conservando la aserción de que `422` gana sobre `409` y `force` no saltea el guard.
- [ ] **2.3** Actualizar el comentario de `TestStampExcludeDates_DiaCerradoExcluidoNoDispara422` para reflejar que hoy async ya no es un día cerrado; verificar que el caso sigue pasando por ayer.
- [ ] **2.4** Correr los tests focales y la suite de `services` con Postgres real; commit `test(calendar): cubrir escritura sobre el dia async de hoy (Gap 39)`.

### Task 3: Documentación y verificación final

**Files:**
- Modify: `docs/CATALOGO_Y_CALENDARIO.md` §8.3
- Modify: `docs/FRONTEND_IMPACTO_INSTANCIACION.md` §4

- [ ] **3.1** Actualizar la tabla y el texto del guard: async de hoy ya no está cerrado; aclarar que lectura/banners conservan su criterio y que aplica a las seis operaciones de escritura.
- [ ] **3.2** Corregir comentarios de código que afirmen que el criterio de lectura espeja el guard de escritura, sin cambiar comportamiento.
- [ ] **3.3** `openspec validate gap-39-async-hoy-asignable --strict`; correr suite completa con Postgres real, `go build`, `go vet`, `gofmt` y `git diff --check`.
- [ ] **3.4** `go clean -cache` + `make coverage-with-db` + analyzer (gate 85); commit `docs(calendar): documentar dia async de hoy escribible (Gap 39)`.
