# Tasks

## Restricciones globales

- Trabajar en la rama actual `fix/team-removal-group-memberships`; no mergear.
- No cambiar contrato HTTP, DTOs, JSON, status codes, autorización, orden visible, timezone, errores ni soft-delete.
- No tocar frontend, Swagger ni schema/migraciones/índices.
- Preservar la semántica de membresía activa: `deleted_at IS NULL` y (`date_end IS NULL OR date_end > NOW()`).
- Sumar tests de paridad y de cantidad de llamadas; los tests de DAO corren contra Postgres local `:5433` con `TEST_DB_*`.
- Coverage total no debe bajar de 85%. Stagear solo rutas explícitas; no `git add -A` ni `git add .`; no stagear `.superpowers/`.
- Al terminar, commit y push según instrucción vigente del usuario; el usuario mergea.

## 1. Calendario

- [x] 1.1 Optimizar `NextSession` para consultar los días primero y resolver nombres de grupo/equipo solo para los `group_id` distintos que aparecen en la respuesta, preservando payload y orden. Verificar con `go test ./cmd/api/services -run 'TestCalendarService_NextSession'` y un test que asegure que no se consultan nombres de grupos no devueltos.
- [x] 1.2 Optimizar `MemberCalendar` para resolver nombres de grupo/equipo solo para los grupos presentes en los días devueltos, sin cambiar visibilidad ni orden. Verificar con `go test ./cmd/api/services -run 'TestCalendarService_MemberCalendar'` y un test de paridad/call-count sobre grupos sin días.
- [x] 1.3 Optimizar `NextPresencialSession` para resolver el equipo/nombre solo del día seleccionado, en vez de todos los equipos owner. Verificar con `go test ./cmd/api/services -run 'TestCalendarService_NextPresencialSession'` y un test que pruebe que no se consultan equipos ajenos al día.
- [x] 1.4 Eliminar la lectura duplicada de `FindByGroupAndDate` antes de la transacción en `UpsertDay`, conservando una única lectura dentro de la transacción y el mismo resultado/errores. Verificar con `go test ./cmd/api/services -run 'TestCalendarService_UpsertDay'` y un test de call-count.

## 2. Sesión presencial y sesión del corredor

- [x] 2.1 Agregar una consulta/DAO que resuelva día + grupo + equipo + owner en una sola operación y usarla en `isDayTeamOwner`, gate/open/close; reemplazar el update + reread por `UPDATE ... RETURNING`. Verificar con `go test ./cmd/api/services -run 'TestPresencialSessionService'` y tests de autorización, estados y timestamps.
- [x] 2.2 En `runner_session_service.Create`, usar `HasInstanceAccessBatch` para caller y atleta y evitar la verificación duplicada de acceso/existencia; en `Finish`, usar `UPDATE ... RETURNING` en lugar de releer. Verificar con `go test ./cmd/api/services -run 'TestRunnerSessionService'` y tests de paridad de errores/estados.
- [x] 2.3 Revisar que los mocks de DAOs usados por presencial/runner session implementen los nuevos métodos o expectativas sin relajar autorización. Verificar con `go test ./cmd/api/services -run 'TestPresencialSessionService|TestRunnerSessionService'`.

## 3. Feedback de entrenamiento

- [x] 3.1 En `workout_feedback_service.Create`, usar `HasInstanceAccessBatch` para caller y atleta y combinar en una sola consulta la pertenencia del ejercicio a la instancia + derivación del team; preservar el orden de validación y los errores actuales. Verificar con `go test ./cmd/api/services -run 'TestWorkoutFeedbackService_Create'` y tests de paridad/call-count.
- [x] 3.2 En `workout_feedback_dao.Update`, reemplazar el update + `GetByID` por `UPDATE ... RETURNING`, manteniendo el shape y los errores. Verificar con `go test ./cmd/api/services -run 'TestWorkoutFeedbackService_Update'` y el test de DAO correspondiente.
- [x] 3.3 Revisar `CreatePoints`/`GetBySession` y dejar constancia de que ya son batch/idempotentes; agregar solo un test de regresión si falta cobertura de no-N+1. Verificar con `go test ./cmd/api/services -run 'TestWorkoutFeedbackService_.*Points|TestWorkoutFeedbackDao_GetBySession'`.

## 4. Verificación final

- [x] 4.1 Ejecutar `go test -count=1 ./...` con `TEST_DB_*` apuntando a `localhost:5433`, `go vet ./...`, `go build ./...`, `openspec validate optimizaciones-calendario-sesion-feedback --strict` y `git diff --check`; confirmar suite verde, build/vet limpios, OpenSpec válido y diff sin whitespace.
- [x] 4.2 Ejecutar la medición de coverage del repo y confirmar que el total no baja de 85%; documentar el resultado en el reporte final.
