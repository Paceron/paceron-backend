# Design — session-registration-review

## Tabla `runner_session`

```sql
CREATE TABLE IF NOT EXISTS runner_session (
    id                  BIGSERIAL PRIMARY KEY,
    session_instance_id BIGINT NOT NULL,
    athlete_user_id     BIGINT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'wip',  -- wip | finished
    start_date          TIMESTAMPTZ NOT NULL,
    end_date            TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (session_instance_id, athlete_user_id)
);
```

### Decisiones

- **FK opacas a `session_instances` y `users`** (sin constraints, ni en struct ni en raw SQL):
  consistente con `workout_feedback.assigned_session_id` — la integridad la gobierna el service
  (validación de que la sesión existe y del atleta), y las constraints reales llegarán en el change
  futuro de las tablas `assigned_*`. El UNIQUE sí es constraint real (no opaco): lo emite
  AutoMigrate vía el tag `uniqueIndex` en el struct GORM (mismo mecanismo que
  `idx_group_calendar_day_group_date`), no raw SQL — es un UNIQUE plano, no parcial.
- **No soft-delete**: el lifecycle `wip → finished` no tiene borrado por ahora; si a futuro se
  quiere reabrir, se agrega en un change aparte.
- **`status` solo vale `'wip'`/`'finished'`**: validado en el service (no CHECK en DB — se sigue el
  patrón de `completion_status` del feedback, validación de app, no de base).

## Idempotencia del create

`Create(ctx, sessionInstanceID, athleteUserID, startDate)`:

```go
res := db.Clauses(clause.OnConflict{
    Columns:   []clause.Column{{Name: "session_instance_id"}, {Name: "athlete_user_id"}},
    DoNothing: true,
}).Create(&runnerSession)   // RowsAffected == 1 si insertó
```

- `RowsAffected == 1` → insertó, devolver el modelo (con `ID`) → el controller responde **201**.
- `RowsAffected == 0` → ya existía → `GetBySessionAndAthlete` y devolver lo actual → el controller
  responde **200** con `{ data: fila }` (mismo shape, status actual; nunca pisa `start_date` ni baja
  de `finished` a `wip`).
- Antes de insertar, el service resuelve la sesión (`session_instance_id` existente → 404 si no) —
  mirror del módulo de feedback que resuelve el feedback antes de operar.

## Finish idempotente

`Finish(ctx, sessionInstanceID, athleteUserID)`:

- `GetBySessionAndAthlete` primero. `ErrRunnerSessionNotFound` → 404.
- Si ya `finished` → devolver la fila tal cual (200, sin reescribir `end_date`).
- Si `wip` → `UPDATE ... SET status='finished', end_date=now(), updated_at=now()
  WHERE id=? AND status='wip'` y devolver la fila atualizada (el service setea `now()` del servidor
  y delega DAO update; o el DAO hace el UPDATE y recarga). Decisión: el **servidor** es dueño de
  `end_date` (el cliente nunca la manda) — evita clocks skew y coincide con la asunción de la spec
  del frontend.

## Autorización (ambos submódulos)

Matriz `atleta dueño | entrenador`:

- Sin `athlete_user_id` → el atleta es `auth_user_id` (self); siempre ok.
- Con `athlete_user_id != auth` → `ExistsUserInTeamOwnedBy(ctx, athleteUserID, authUserID)` (¿el
  auth es owner de un equipo al que pertenece el atleta?) → si no, `ErrRunnerSessionForbidden` (403).
  Misma función que usa `workoutFeedbackService.Create` para su matriz. El `RunnerSessionDAO`
  expone los métodos de membresía delegando al `TeamMembershipDao`, igual que
  `workoutFeedbackDao`.

## Validaciones

- `id` de sesión (path): entero > 0 (reusa `parsePositivePathParam`).
- `start_date`: RFC3339 parseable, obligatorio en POST (si no viene → 400).
- `status` en PATCH: debe ser exactamente `"finished"` (cualquier otro valor → 400).
- `athlete_user_id` (body/query): entero > 0 si viene; ajeno → check de trainer.

## Feedback por sesión

`GetSessionFeedback(ctx, authUserID, sessionInstanceID, athleteUserID *int64)` en el
`workout_feedback_service`:

- Resolver el atleta objetivo (default self) y aplicar la matriz (self ok; ajeno → trainer check).
- Delegar en `Search` con `AssignedSessionID = &sessionInstanceID`, ordenado por el DAO
  (`query.Order("id")` — el orden natural de creación refleja `(assigned_exercise_id, set_number)`
  porque el frontend inserta en ese orden; para garantizar el orden exacto del spec se agrega
  `Order("assigned_exercise_id, set_number, id")` al DAO, ver tasks 3.3).

Espera — el `Search` actual ordena por `id` sólo. Para cumplir el requisito de orden
`(assigned_exercise_id, set_number)` se agrega un método DAO dedicado `GetBySession` en
`workout_feedback_dao.go` que filtra por `assigned_session_id` (+ `athlete_user_id` opcional) y
`deleted_at IS NULL` y ordena `(assigned_exercise_id, set_number, id)`. El `Search` existente queda
intacto (otra versión de `GetBySession`). El service lo usa directamente, sin pasar por `Search`.

## Respuestas

- `POST runner` → `201 { message, data: RunnerSessionResponse }` (inserción) o
  `200 { message, data: RunnerSessionResponse }` (ya existía).
- `PATCH runner` → `200 { message, data: RunnerSessionResponse }`.
- `GET runner` → `200 { data: RunnerSessionResponse }` | `404`.
- `GET feedback` → `200 { data: [WorkoutFeedbackResponse] }` ordenado por `(assigned_exercise_id,
  set_number)`; sin filas → `200 { data: [] }`.

```go
type RunnerSessionResponse struct {
    ID                int64     `json:"id"`
    SessionInstanceID int64     `json:"session_instance_id"`
    AthleteUserID     int64     `json:"athlete_user_id"`
    Status            string    `json:"status"`
    StartDate         time.Time `json:"start_date"`
    EndDate           *time.Time `json:"end_date"`
}
```

## Rutas

```go
r.POST("/api/v1/session-instances/:id/runner", app.runnerSessionController.Create)
r.PATCH("/api/v1/session-instances/:id/runner", app.runnerSessionController.Finish)
r.GET("/api/v1/session-instances/:id/runner", app.runnerSessionController.Get)
r.GET("/api/v1/session-instances/:id/feedback", app.workoutFeedbackController.GetBySession)
```

Fila de registros no guarda colisión de globs (Gin prioriza literales/segmentos fijos; no hay una
ruta `:id` genérica en `/session-instances` con la que convivir hoy) — aun así se registra el
`feedback` junto al resto de rutas de `session-instances` para mantener el orden legible.

## Fuera de alcance

- Historial/borrado de actividades; edición de GPS/RPE/repeticiones en la revisión; asistencia en
  vivo; soft-delete de `runner_session`.