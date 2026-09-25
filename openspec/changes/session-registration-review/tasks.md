# Tareas — session-registration-review

## 1. Modelo y migración

- [ ] 1.1 Crear `cmd/api/domains/dbs/runner_session.go` con `RunnerSession`:
      `session_instance_id` + `athlete_user_id` con `uniqueIndex:uq_runner_session_session_athlete`
      (priority 1/2), `status` default `'wip'`, `start_date` not null, `end_date`, `created_at`/
      `updated_at` auto. `TableName()` → `runner_session`.
- [ ] 1.2 Agregar `&dbs.RunnerSession{}` al `AutoMigrate` de
      `cmd/api/infrastructure/postgresdb/postgres.go`.
- [ ] 1.3 Verificar que AutoMigrate cree el constraint único `uq_runner_session_session_athlete`.

## 2. DTOs

- [ ] 2.1 Crear `cmd/api/domains/runnersession/runner_session.go`:
      `CreateRunnerSessionRequest { AthleteUserID *int64; StartDate time.Time (json, required) }`,
      `RunnerStatusRequest { Status string (json, required) }`,
      `RunnerSessionResponse { ID, SessionInstanceID, AthleteUserID, Status, StartDate, EndDate }`,
      `MutationResponse { Message string; Data *RunnerSessionResponse }` (mensajes: "estado de
      sesión creado", "sesión marcada como completada").
- [ ] 2.2 Constantes `MsgRunnerSessionCreated` / `MsgRunnerSessionFinished`.

## 3. DAO runner_session

- [ ] 3.1 Crear `cmd/api/daos/runner_session_dao.go` con
      `RunnerSessionDAOInterface`:
      - `Create(ctx, r *dbs.RunnerSession) (created bool, err error)` — `clause.OnConflict{
        Columns: [session_instance_id, athlete_user_id], DoNothing: true }`; devuelve
        `RowsAffected == 1`.
      - `GetBySessionAndAthlete(ctx, sessionInstanceID, athleteUserID) (*dbs.RunnerSession, error)` —
        `ErrRunnerSessionNotFound` si no existe.
      - `Finish(ctx, r) error` — `UPDATE ... SET status='finished', end_date=?, updated_at=? WHERE
        id=? AND status='wip'` — solo marca si estaba en `wip` (el service decide con el GET previo;
        el UPDATE actúa como guard extra).
      - Métodos de membresía: `TeamExists`, `IsTeamOwner`, `ExistsUserInTeamOwnedBy` (delegando a
        `TeamMembershipDao`, patrón de `workoutFeedbackDao`).
- [ ] 3.2 Errores: `ErrRunnerSessionNotFound = errors.New("estado de sesión no encontrado")`.
- [ ] 3.3 `cmd/api/daos/workout_feedback_dao.go`: agregar `GetBySession(ctx, sessionInstanceID,
      athleteUserID *int64) ([]dbs.WorkoutFeedback, error)` con `assigned_session_id = ?`,
      `athlete_user_id = ?` si viene, `deleted_at IS NULL`, orden
      `assigned_exercise_id, set_number, id` + expansión de la interfaz (NO tocar `Search`).

## 4. Service runner_session

- [ ] 4.1 Crear `cmd/api/services/runner_session_service.go` con
      `RunnerSessionServiceInterface`:
      - `Create(ctx, authUserID, sessionInstanceID, req) (*dbs.RunnerSession, created bool, error)`:
        valida `session_instance_id` (resolver sesión existente — 404 si no), `start_date` no zero,
        resuelve atleta (self o trainer-check con `ExistsUserInTeamOwnedBy`), llama DAO de session
        `Create`; si `created=false` devuelve el actual.
      - `Finish(ctx, authUserID, sessionInstanceID, req) (*dbs.RunnerSession, error)`: valida
        `status == "finished"`, resuelve atleta, `GetBySessionAndAthlete` (404), si ya `finished`
        devuelve tal cual (idempotente), si `wip` → setea `end_date = now()` del servidor y delega
        `Finish` del DAO, re-carga y devuelve.
      - `Get(ctx, authUserID, sessionInstanceID, athleteUserID *int64) (*dbs.RunnerSession, error)`.
      - Errores: `ErrRunnerSessionInvalid` (400), `ErrRunnerSessionForbidden` (403).
      - Chequeo de sesión existente: `sessionInstanceExists(ctx, id)` delegando a una consulta
        simple en el DAO (`SELECT 1 FROM session_instances WHERE id=?`) — agregar
        `SessionInstanceExists(ctx, id) (bool, error)` al `RunnerSessionDAO`.
- [ ] 4.2 `cmd/api/services/workout_feedback_service.go`: `GetSessionFeedback(ctx, authUserID,
      sessionInstanceID, athleteUserID *int64) ([]dbs.WorkoutFeedback, error)` — resuelve atleta
      (self o trainer-check), delega en dao.GetBySession, devuelve `[]` si nil. + interfaz.

## 5. Controllers y rutas

- [ ] 5.1 Crear `cmd/api/controllers/runner_session_controller.go` (`Create` 201/200, `Finish` 200,
      `Get` 200/404) con `GET_AuthUserID`, `parsePositivePathParam("id")`, parseo de
      `athlete_user_id` opcional del body/query, `respondRunnerSessionError` (400/401/403/404).
- [ ] 5.2 `workout_feedback_controller.go`: `GetBySession` (200/400/401/403), reusa
      `respondFeedbackError` + `toWorkoutFeedbackResponse` (para `data` usa el shape de lista).
- [ ] 5.3 Swagger annotations godoc para las 4 rutas.
- [ ] 5.4 DI en `cmd/api/app/app.go`: `runnerSessionDao := daos.NewRunnerSessionDao(db)`,
      `runnerSessionService := services.NewRunnerSessionService(runnerSessionDao)`,
      `runnerSessionController := controllers.NewRunnerSessionController(runnerSessionService)`;
      el `workoutFeedbackService` ya existe (agregar `GetSessionFeedback` a su interfaz).
- [ ] 5.5 Registrar en `cmd/api/app/url_mappings.go`:
      `POST/PATCH/GET /api/v1/session-instances/:id/runner` y
      `GET /api/v1/session-instances/:id/feedback`.
- [ ] 5.6 Regenerar swagger (`make swagger` o el comando del repo).

## 6. Tests

- [ ] 6.1 DAO test `runner_session_dao_test.go` (mocks/GORM real según patrón del módulo):
      create inserta; create con fila existente no duplica (created=false); get found/not-found;
      finish sobre `wip` marca; sobre `finished` no reescribe `end_date`.
- [ ] 6.2 `workout_feedback_dao_test.go`: `GetBySession` filtra/ordena y excluye soft-deleted.
- [ ] 6.3 Service test `runner_session_service_test.go`: sesión inexistente (404), start_date vacío
      (400), atleta ajeno sin relación (403) / con relación (ok), idempotencia de create y de
      finish, end_date seteada por servidor.
- [ ] 6.4 Service test feedback: matriz self/trainer/no-auth + orden.
- [ ] 6.5 Controller test: 200/201/400/403/404 de las 4 rutas.
- [ ] 6.6 `go test ./...` verde; cobertura del repo >= 85%.

## 7. Docs

- [ ] 7.1 Swagger regenerado en `cmd/api/docs/`.
- [ ] 7.2 Nota en `CLAUDE.md` del backend: `runner_session` = estado de dominio del corredor
      (wip→finished), FK opaca + UNIQUE real, creación idempotente.
- [ ] 7.3 Copiar spec aprobada a `openspec/specs/session-registration-review/spec.md`.