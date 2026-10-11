# Diseño: latlng-optimizaciones-y-endurecimiento

> Fuente de verdad de los valores: `cmd/api/domains/` + `cmd/api/daos/`.
> Todas las verificaciones de este doc se hicieron contra `develop 6f29c47`.

## Decisiones (D1–D10)

### Gap 29 — ubicación con lat/lng

**D1 (modelo y columnas).** `dbs/user.go` y `dbs/team.go` suman
`Lat *float64 gorm:"column:lat"` y `Lng *float64 gorm:"column:lng"` (nullable,
sin backfill: filas viejas NULL). Cultural check hecho: `trainingplan.Location
{Lat, Lng float64}` ya usa `float64` como precedente de tipo.

**D2 (contrato, aditivo).** Acepta/devuelve `lat`/`lng` opcional (igual que
`country`/`province`/`city`):

- registro (`RegisterRequest` + `RegisterResponse`/auth user responses),
- `UpdateUserRequest`/user update response,
- `CreateTeamRequest`/`UpdateTeamAddressRequest`/`TeamResponse` (todos los
  DTOs que hoy reflejan dirección: `GET /teams/{id}`, GetAll, Create, Update,
  UpdateAddress).

`TeamSearchResult` NO cambia (no expone dirección hoy — 0 cambios en el
autocompletado del buscador). Validación de rango en el service: `lat ∈
[-90, 90]`/`lng ∈ [-180, 180]` con margen de error; fuera de rango → `400`
con el patrón de mensaje de campo inválido acostumbrado ("lat fuera de
rango [-90, 90]" o lo que dicte el review del estilo del paquete, decisión
fija en tests).

**D3 (charset de dirección, veracidad de errores).** Se amplían los regex
existentes (aditivo, solo agrega caracteres admitidos):

- `user_service.go:474-498` (update: country/city/province/street/number) y
  `auth_service.go:471-489` (registro: country/city/street/number): agregar
  `-` (guion), `.` (punto), `'` (apóstrofe), `,` (coma) al conjunto permitido.
- Los mensajes de error se actualizan para LISTAR los caracteres nuevos
  ("country solo acepta letras, espacios, guiones, puntos, apóstrofes y
  comas" con las variantes por campo) — es la única alteración de texto de
  respuesta; mentir el charset era un defecto no funcional previo. Motivo:
  Nominatim resuelve nombres con guiones/puntos/apóstrofes (ej. "O'Higgins",
  "Villa Elisa–km 12").
- Teams y el resto de campos no acepta charset (hoy no lo validan; queda
  igual — se decidió no endurecer sin pedido).

**D4 (Swagger).** Anotar lat/lng en cada endpoint tocado; regenerar con el
comando canónico del repo `swag init --parseDependency -g cmd/api/docs.go
--output cmd/api/docs` (NO -g main.go con --parseInternal; arrastra churn).

### Rendimiento — escritura y lectura del calendario

**D5 (upsert multi-fila `UpsertMany` y semántica presencial).** Nuevo método
DAO `UpsertMany(ctx, days []dbs.GroupCalendarDay) error`, 1 query por lote:

```sql
INSERT INTO group_calendar_days (group_id, date, kind, other_name,
  session_instance_id, cancelled_reason, is_presencial,
  presencial_time_from, presencial_time_to, presencial_location,
  source_plan_id, source_session_id...)  -- lat/lng no aplica acá
VALUES (...), (...)
ON CONFLICT (group_id, date) DO UPDATE SET kind=EXCLUDED.kind, ...
```

Con `RETURNING id` (gorm `clause.OnConflict{Columns: [...], DoUpdates:
clause.AssignmentColumns([...])}` + `clause.Returning{}`; el orden del
RETURNING matchea el orden del VALUES: se testa con un test que garantice
el mapeo índice→id).

La lógica Gap-26 de limpieza presencial (de PR #93: si
`!sameNullableInt(existing.SessionInstanceID, EXCLUDED.session_instance_id)
|| NOT EXCLUDED.is_presencial` → `opened_at`/`closed_at` a NULL) se portea
a SQL: `presencial_opened_at = CASE WHEN <condición> THEN NULL ELSE
group_calendar_days.presencial_opened_at END` (usando columnas de la fila
actual + EXCLUDED, referenciables en el DO UPDATE). El `sameNullableInt`
de Go queda para el upsert de fila única.

**D6 (instanciación batch y borrado batch en el write path de
stamp/bulk/shift).**

- Lectura de catálogo por lote (1 round por tabla): sesiones para los
  `sessionID` distinct (`sessionDao.FindByIDs` si existe, si no se agrega),
  links de catálogo `session_exercises WHERE session_id IN ?` y ejercicios
  de catálogo (`FindByIDs` existente). `ErrCalendarSessionNotFound`
  preservado: si una sesión de catálogo falta al instanciar el día, sale el
  mismo sentinel con el mismo mensaje (el review fija si el mensaje lleva
  el id del día o solo de la sesión — hoy lleva id de sesión, se replica
  tal cual).
- Instancia de sesión por DÍA (D4 no deduplica): batch INSERT de
  `session_instances` (una fila por (día, session) con `source_session_id`
  — Gap 7) + RETURNING ids; se construye el map índice→instanceID antes
  del upsert.
- Ejercicio/links por DÍA: batch INSERT de `exercise_instances` (una fila
  por (día, link), copiando el catálogo ya leído en memoria) + RETURNING
  ids; después batch INSERT de `session_exercise_instances` (con
  role/repeat_count/rest_minutes del catálogo + `source_exercise_id`).
  Nunca deduplicar ejercicios entre días.
- Borrado de superadas batch (`deleteSupersededInstances(ctx, tx, ids)` —
  plural de `deleteSupersededInstance`, que queda para el upsert de fila
  única): (a) 1 query feedback por sesión (`IN ?`), (b) 1 query links de
  las NO protegidas (`FindBySessionInstances` de la PR #97), (c) 1 query
  feedback por ejercicio (`assigned_exercise_id IN ?` de los links),
  (d) Deletes con `IN` de links/jercicios/sesiones de las instancias
  realmente borrables. Semántica D10 por instancia idéntica (feedback de
  sesión O de cualquier ejercicio protege la instancia entera). El review
  fija: la implementación puede optar por (a) una sola query combinada con
  OR `session/exercise` si el review acepta la reescritura del filtro de
  feedback, o por la descomposición en 3 queries acá descrita (decisión de
  implementación, se asenta en report).

**D7 (bulk-clear).** Del loop `N × FindByGroupAndDate` (que además hace el
closed-check por fecha contra la fila del día) a: 1 query
`WHERE group_id = ? AND date IN ?` (métodos nuevos DAO; si las fechas son
contiguas por diseño, se puede usar `FindByGroupAndRange`, pero `IN` cubre
ambos y es lo propuesto), guards de cerrado en memoria con
`closedDayForRequest`/`isCalendarDayClosed` (mismo criterio), 1
`DeleteByDates`, y supersede batch de la D6 con las instancias de las
filas leídas. Errores/lista de fechas del `422` en el mismo orden de hoy.

**D8 (administered-calendar, colisiones por rango).** Del “1
`findPresencialCollisions` por fecha con ≥2 presenciales” a: 1 query de
días presenciales del owner para el rango completo, y clasificación/
atribución/dedup cross/same EN MEMORIA con las mismas reglas del guard de
la PR #76 (overlap medio-abierto, cross antes de dedup, `cancelled`
excluido, re-attribución por día). El helper existente queda intacto para
el write path (PUT/bulk/stamp/shift). Solo lectura; presupuesta el shape
actual `presencial_collision {type, conflicts}` byte-idéntico (incluidas
las colisiones viejas de antes del guard).

**D9 (invitaciones sin N+1).** `ListPendingInvitationsForUser` (y la ruta
por team si tiene el mismo patrón): dedupe de team ids (team `FindByIDs`
existente — el flag `can_receive_payments` de la PR #96 ya usa batch en
GetAll) y de user ids (invitee + inviter, `FindByIDs` existente) → 3
queries. `toInvitationResponse` modifica su firma a ( invitation, teamMap,
userMap ) o recibe los resueltos desde el service. Miss de map (equipo
borrado, usuario inexistente) → mismos valores nil/“” que el `FindByID`
devuelve hoy para no encontrar (verificado en el review).

**D10 (seguridad).** Tres frentes de tests valiosos (no es un linter):

1. **Pins anti-inyección** en los filtros de texto que terminan en SQL:
   `q` de `GET /users/search`, `country`/`province`/`city` de
   `GET /teams/search`, filtros de texto del historial (Gap 13), y los
   sort/order whitelisted (Probar con sort fuera de whitelist también =
   fallback). Payload estilo `'; DROP TABLE x;--` matcheado como literal
   (200 con resultado vacío o el matcheo textual, sin error de SQL,
   sin escalar).
2. **No-leak en 500s**: con el helper `FailingDB` (`cmd/api/testutils/`
   — inyecta fallos por TIPO de operación SQL en el `handle gorm`),
   asertar que las respuestas 500 genéricas de los controllers NO contienen
   el `err.Error()` del driver (`SQLSTATE`, `syntax error`, detalles de
   Postgres); usar tests de endpoints con mock de service o pools de
   fallo ya existentes como base, sin cambiar mensajes vigentes.
3. **Audit de authz**: relevar los endpoints y sus guards (owner,
   pertenencia-as-team, self-only, dual HasInstanceAccess) contra los
   tests existentes de controller; Sumar tests 403/404 donde falten
   (con el patrón de mock del paquete controllers). Salida de la tarea:
   tabla endpoint → guard → test que lo cubre (report del implementador),
   más los tests nuevos “cierre” que correspondan. JWT edge cases ya
   cubiertos (WS 401 sin/inválido/expirado + REST); NO se re-testea.

## Verificación (transversal)

- Postgres real (`docker start paceron-test-db`, :5433,
  `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres
  TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`).
- `openspec validate latlng-optimizaciones-y-endurecimiento --strict` →
  valid.
- Suite completa `go test -count=1 ./...` → 0 FAIL. Aserciones de las
  rutas optimizadas SIN cambiarlas (solo mocks por interfaces extendidas).
- Coverage: `go clean -cache` + `make coverage-with-db` + analyzer
  (`go run github.com/vladopajic/go-test-coverage/v2@latest --config
  ./.testcoverage.yml --profile ci/test_coverage/coverage.out`) → gate 85
  PASS, `.testcoverage.yml` sin diff. Bug de cache conocido: sin clean el
  número puede salir falso ~70-72%.
- gofmt/build/vet limpios en los archivos tocados; Swagger regenerado
  solo para el D1/D2 (comando canónico); respuestas del calendario sin
  diff de shape vs develop.
