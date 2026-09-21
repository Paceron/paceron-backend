## Tasks

### Task 1: Columnas de origen en modelos y referencia app-managed

- [ ] 1.1 Agregar `SourceSessionID *int64` (`column:source_session_id`) a `dbs.SessionInstance` y `SourceExerciseID *int64` (`column:source_exercise_id`) a `dbs.ExerciseInstance`, con comentario corto (referencia opaca informativa, patrón `source_plan_id`, nunca se limpia).
- [ ] 1.2 Poblarlas en `instantiateSession` (`calendar_service.go`): sesión = id de la `Session` de catálogo resuelta; cada ejercicio = id del `Exercise` copiado.
- [ ] 1.3 `go build ./...` (AutoMigrate crea las columnas sin cambios explícitos; verificar que ningún `Create` de DAO las pise).

### Task 2: Campos aditivos en respuestas D9

- [ ] 2.1 `SessionInstanceResponse` suma `SessionID *int64` json `session_id`; `InstanceExerciseResponse` suma `ExerciseID *int64` json `exercise_id` (`instance/instance_response.go`).
- [ ] 2.2 Mapearlos en `NewSessionResponse`/`NewExerciseResponse` desde las columnas de origen.
- [ ] 2.3 Tests del mapper (valores presentes y `null` para instancias sin origen — filas creadas a mano con columna nula).

### Task 3: Conservar instancia sin session_id (PUT individual y bulk)

- [ ] 3.1 `validateDayFields` recibe `hasExistingInstance bool`; rama `training`: error solo si `req.SessionID == nil && !hasExistingInstance`. Callers: `UpsertDay` y `Bulk` (loop de validación por fecha) calculan el flag de `existing`/`txExisting` correspondiente.
- [ ] 3.2 Transacción de `UpsertDay`: `kind=training` con `session_id == nil` → `row.SessionInstanceID = txExisting.SessionInstanceID` y flag `preservada`; extender la condición de `deleteSupersededInstance` con `&& !preservada`. Espejo en el path mock (`s.db == nil`) igual que el de `cancelled`.
- [ ] 3.3 `Bulk`: en el loop de escritura, `kind=training` con `session_id == nil` → conservar `existing[i].SessionInstanceID` y skip `deleteSupersededInstance` por fecha; nuevo error `ErrCalendarTrainingWithoutInstance` (lista fechas sin instancia, `422`, all-or-nothing — mismo patrón que `newCalendarClosedDaysError`, mapeo en `mapCalendarError`/controller). Espejo path mock.
- [ ] 3.4 Ajustar anotaciones Swagger del `PUT` individual y `bulk` (doc `session_id` condicional/conservación) y `swag init` para regenerar `cmd/api/docs`.

### Task 4: Tests de servicio y DAO

- [ ] 4.1 DAO/Postgres real: `instantiateSession` persiste `source_session_id`/`source_exercise_id` correctos.
- [ ] 4.2 Service real-Postgres: PUT training sin `session_id` sobre día con instancia conserva el mismo `session_instance_id`, no crea ni borra filas de instancia, y la respuesta embebe `session_id`/`exercise_id` de origen.
- [ ] 4.3 Service: PUT training sin `session_id` sobre día sin instancia → `422` `ErrCalendarFieldMismatch`.
- [ ] 4.4 Bulk real-Postgres: sin `session_id` sobre N fechas con instancia → cada una conserva la suya, cero filas nuevas/borradas; con una fecha sin instancia → `422` con la lista de fechas y ninguna fila modificada (all-or-nothing).
- [ ] 4.5 Regresión: PUT con `session_id` presente sigue reinstanciando y borrando/conservando huérfana según feedback (los tests existentes de Task 3/5 del change anterior deben seguir verdes sin cambios semánticos).
- [ ] 4.6 Instancia con origen `NULL` (fila creada directo en DB) responde `session_id: null`/`exercise_id: null` sin error.

### Task 5: Documentación y verificación final

- [ ] 5.1 `docs/CATALOGO_Y_CALENDARIO.md` §8: documentar columnas de origen + conservación en PUT individual y bulk.
- [ ] 5.2 `docs/FRONTEND_IMPACTO_INSTANCIACION.md`: nota de cambio aditivo (Gap 7 cerrado, qué puede hacer el frontend ahora; sin acción obligatoria para clientes viejos).
- [ ] 5.3 `openspec validate instancia-referencia-catalogo --strict`, `gofmt`, `go build ./...`, `go vet ./...`, `go test ./...` con Postgres real, `make coverage-with-db` (gate 80 sin tocar el umbral).
