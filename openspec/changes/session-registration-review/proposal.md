## Why

El registro en vivo del frontend (specs `2026-09-23-live-session-base-design.md` y
`2026-09-24-live-session-recording-design.md`) persiste un `workout_feedback` por serie al finalizar
una sesión, pero no hay forma de saber si la sesión quedó **completada** ni de volver a **ver/editar**
lo registrado después. El frontend quiere, para toda sesión de fecha pasada, reemplazar el botón
Play del pre-start por un "Registro de Sesión" (badge de completada o carga manual), y una pantalla
para revisar/editar el feedback de cada serie — por el corredor (mobile y web) y por el entrenador
(web). Ver `paceron-frontend/docs/superpowers/specs/2026-09-24-session-registration-review-design.md`.

Para eso hacen falta (a) una tabla de **estado de sesión del corredor** (`runner_session`,
`wip → finished`) creada la primera vez que se da Play o se entra al registro manual — idempotente,
nunca duplicada —, y (b) endpoints de lectura de feedback por sesión (`GET feedback` por símbolo de
sesión) que la pantalla de revisión consume. La edición puntual de un feedback ya existe
(`PUT /workout-feedback/:id`); el alta manual reusa el `POST /workout-feedback` existente.

## What Changes

- **Tabla `runner_session`**: `id BIGSERIAL PK`, `session_instance_id BIGINT NOT NULL` (FK opaca a
  `session_instances`, sin constraint — mismo patrón de `workout_feedback`), `athlete_user_id BIGINT
  NOT NULL` (usuario atleta), `status TEXT NOT NULL DEFAULT 'wip'` (`wip | finished`),
  `start_date TIMESTAMPTZ NOT NULL`, `end_date TIMESTAMPTZ`, `created_at`/`updated_at`.
  `UNIQUE (session_instance_id, athlete_user_id)` — un corredor tiene un solo estado por sesión
  asignada; dos atletas pueden compartir la misma sesión (día de grupo).
- **`POST /api/v1/session-instances/:id/runner`** — crea el estado en `wip`, **idempotente**
  (`ON CONFLICT DO NOTHING`; si la fila ya existe responde `200` con el estado actual, sin pisar
  `start_date` ni bajar de `finished` a `wip`). Body `{ athlete_user_id?, start_date }`: atleta
  default = self; si viene un atleta ajeno, requiere que el auth sea owner de un equipo del atleta.
- **`PATCH /api/v1/session-instances/:id/runner`** — `{ status: 'finished' }`: pasa a `finished` y
  setea `end_date = now()` de servidor solo si estaba en `wip`; idempotente (ya `finished` → `200`
  con el estado actual); `404` si no hay fila. Misma autorización.
- **`GET /api/v1/session-instances/:id/runner`** — estado actual; `?athlete_user_id=` (default
  self). Misma autorización.
- **`GET /api/v1/session-instances/:id/feedback`** — lista los `workout_feedback` de esa sesión,
  `?athlete_user_id=` (default self), ordenados por `(assigned_exercise_id, set_number)`. El
  frontend agrupa por ejercicio contra el shape del `session_instance` (ya conoce los nombres de
  ejercicios); no se hace join a `exercise_name` acá.
- **Autorización de todos**: el atleta sobre sus datos, o un entrenador (owner de un equipo al que
  pertenezca ese atleta — misma lógica `ExistsUserInTeamOwnedBy` del módulo de feedback).

## Capabilities

### New Capabilities

- `session-registration-review`: estado persistido del corredor por sesión (`runner_session`,
  `wip → finished`, creación idempotente) y lectura de feedback por sesión para la pantalla de
  revisión/registro manual, autorizada para el atleta dueño y el entrenador.

## Impact

- Modelos: `domains/dbs/runner_session.go` (GORM, `uniqueIndex` en el par; AutoMigrate crea la
  tabla y el constraint).
- DTOs: `domains/runnersession/runner_session.go` (requests/responses).
- DAO: `daos/runner_session_dao.go` (`Create` idempotente, `GetBySessionAndAthlete`, `Finish`) +
  interfaz; el listado de feedback por sesión reusa el `Search` del `workout_feedback_dao`
  (`AssignedSessionID` + scope self o `AthleteUserID` tras chequear trainer).
- Service: `services/runner_session_service.go` (create/finish/get + matriz de auth, reusando el
  dao de membresía vía `RunnerSessionDAO`) + `GetBySession` en `workout_feedback_service.go`.
- Controller: `controllers/runner_session_controller.go` (3 handlers + swagger) y
  `GetBySession` en `workout_feedback_controller.go`. DI en `app/app.go`, rutas en
  `app/url_mappings.go` (nuevo namespace `/session-instances/:id/...`).
- Swagger: `docs/` regenerado (3 rutas nuevas + listado de feedback por sesión).
- Docs: nota en `CLAUDE.md` del backend (tabla nueva, FK opaca, idempotencia); el frontend ajusta
  la nota de `runner_session` como fuente de verdad del estado.
- Tests: DAO (create idempotente, finish, not found), service (matriz de auth self/trainer/no-auth,
  validaciones), controller (200/201/400/403/404) — `testify`, patrón de
  `workout_feedback_*_test.go`. Cobertura del repo se mantiene >= 85%.

## Non-Goals

- Historial multiplataforma completo (listar actividades de todos los corredores con filtros,
  eliminar) — Gap 12 del frontend; este change solo provee la lectura base por sesión que el
  historial reusará.
- Edición de ruta GPS, RPE/repeticiones/pulso en la pantalla de revisión (el schema ya las
  soporta; el front de esta versión edita solo tiempos/distancia vía el `PUT /workout-feedback/:id`
  existente).
- Asistencia/QR y monitoreo en vivo del entrenador (módulo aparte).
- Soft-delete en `runner_session`: el lifecycle es `wip → finished` sin borrado por ahora.
- FK reales a `session_instances`/`users`: siguen opacas, consistentes con `workout_feedback`.