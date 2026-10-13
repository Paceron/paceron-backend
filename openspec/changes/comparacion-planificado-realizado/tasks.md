## 1. Change de OpenSpec

- [x] 1.1 Crear `proposal.md`, `design.md`, `specs/comparacion-entrenamientos/spec.md` y `tasks.md`.

## 2. DTOs y cálculo

- [x] 2.1 Crear `cmd/api/domains/comparison/activity_comparison.go` con los DTOs del contrato.
- [x] 2.2 Crear `cmd/api/services/activity_comparison_calc.go`: normalización del feedback (D6), cumplimiento y desviación por serie (D4, D5), armado de ejercicio y sesión.
- [x] 2.3 Tests tabla por tabla: tiempo, distancia, sin objetivo, salteada, sin registro, base 0, filas dobles, series extra, división por cero, ritmo.

## 3. DAO de solo lectura

- [x] 3.1 Crear `cmd/api/daos/activity_comparison_dao.go`: días planificados por membresía histórica (D7), sesiones con feedback del rango, día de una instancia, ejercicios planificados, feedback y `runner_session` en lote, y chequeos de dueño de equipo.
- [ ] 3.2 Tests contra Postgres: membresía que termina en el medio del rango, día cancelado, filtros de equipo y grupo, sesión sin día, feedback borrado, lote vacío, errores de DB con `FailingDB`.

## 4. Service

- [x] 4.1 Crear `cmd/api/services/activity_comparison_service.go` con `GetSessionComparison` y `ListComparisons`: validación, acceso (D3) y armado de la respuesta.
- [x] 4.2 Tests con mocks: acceso propio y de entrenador, 404, 403, rango inválido, `kind` inválido, filtro por tipo, sesiones sin día, resumen, errores del DAO.

## 5. Controller, wiring y rutas

- [x] 5.1 Crear `cmd/api/controllers/activity_comparison_controller.go` con godoc de swagger.
- [x] 5.2 Wiring en `app/app.go` y rutas en `app/url_mappings.go`.
- [x] 5.3 Tests con `httptest`: 200, 400, 401, 403, 404 y 500.

## 6. Swagger y documentación

- [ ] 6.1 Regenerar swagger.
- [ ] 6.2 Agregar los endpoints al `README.md` y una sección en `docs/CATALOGO_Y_CALENDARIO.md`.
- [ ] 6.3 Registrar en `docs/DEUDA_TECNICA_Y_PENDIENTES.md`: `source_plan_id` que se pierde al editar un día, `set_number` en base 0 de las filas viejas y el comentario desactualizado de `dbs/workout_feedback.go`.

## 7. Verificación

- [ ] 7.1 `go vet ./...` y `go test ./...`.
- [ ] 7.2 `make coverage-with-db`: total de 85% o más y archivos nuevos de 90% o más.
- [ ] 7.3 Probar contra la base de testing con la ejecución sembrada del 2026-10-12 (sesión 455, corredor 19): 50,1%.
