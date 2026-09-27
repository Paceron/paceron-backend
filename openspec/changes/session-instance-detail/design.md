# Diseño — session-instance-detail

## Decisiones

**D1 — Reuso del builder.** `CalendarService.sessionInstanceResponse(ctx, db, id)` ya arma `SessionInstanceResponse` a partir del id (instance → links → exercises → `instance.NewSessionResponse`, con error si falta un ejercicio instancia). El nuevo service method lo invoca sobre `s.db`; `404` cuando la instancia no existe (el helper devuelve error de "no encontrada" — distinguir de un error de DB para mapear códigos HTTP).

**D2 — Autorización dual (Opción B).** Método de acceso `hasInstanceAccess(ctx, db, instanceID, callerID) (bool, error)` con dos existence-checks:
1. `EXISTS group_calendar_day gcd JOIN groups g ON g.id = gcd.group_id JOIN group_users gu ON gu.group_id = g.id AND gu.user_id = caller AND gu.date_end > NOW() AND gu.deleted_at IS NULL WHERE gcd.session_instance_id = ?` — membresía activa del atleta. La variante entrenador: `EXISTS ... JOIN teams t ON t.id = g.team_id AND t.owner_id = caller AND t.deleted_at IS NULL`.
2. `EXISTS workout_feedback wf JOIN teams t ON t.id = wf.team_id WHERE wf.assigned_session_id = ? AND wf.deleted_at IS NULL AND (wf.athlete_user_id = caller OR wf.feedback_owner_user_id = caller OR t.owner_id = caller)`.

Ambas consultas viven en un DAO (existencia pura, sin carga de filas). Pueden combinarse en un solo método `HasInstanceAccess(instanceID, callerID)` que corre 2 queries o 1 con OR; se prefiere 2 queries simples en el mismo DAO para legibilidad. Sin acceso → `403`; instancia inexistente → `404` (el 404 se chequea primero con `FindByID`, así el 403 no revela existencia de ids que no existen).

**D3 — Orden de chequeos.** `FindByID` → 404; `hasInstanceAccess` → 403; build del shape → errores internos (500 vía mapeo existente).

**D4 — Casa del endpoint.** `SessionInstanceDetail(ctx, id, callerID)` en `CalendarServiceInterface` + handler en calendar controller (`SessionInstanceDetail`), ruta plana `r.GET("/api/v1/session-instances/:id", app.calendarController.SessionInstanceDetail)` en `url_mappings.go` junto a las rutas de `/session-instances/:id/*`. Mapeo de errores por `mapCalendarError` + errores tipados nuevos si hace falta (404/403 como sentinels del paquete calendar).

**D5 — Sin cambios en nada existente.** El helper `sessionInstanceResponse`, los endpoints de runner/feedback, y el shape D9 quedan intactos. La ruta nueva no colisiona con los subpaths ya registrados.

**D6 — Testing.** DAO: existence-checks con Postgres real (fixture: grupo del caller con día→instancia; grupo ajeno; huérfana con feedback propio/de otro/owner; soft-deleted membership; feedback soft-deleted). Service: matriz 404/403/200 + shape completo con Postgres real (fixture propia, no reutiliza los de colisiones). Controller: mock del service — parsing id, 200/403/404/401. Verificación final: `go clean -cache` + `make coverage-with-db`, analyzer ≥ gate 85.
