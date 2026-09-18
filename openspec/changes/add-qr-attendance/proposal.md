# Asistencia por QR

## Why

El sistema necesita registrar asistencias de corredores a sesiones de entrenamiento. Hoy no existe ningún concepto de asistencia: el único camino es manual. Un código QR por sesión permite que el corredor se registre escaneándolo desde la app (con su token autenticado), de forma idempotente (un corredor = una asistencia por sesión y por equipo), y que el entrenador (owner del equipo) pueda consultar y buscar asistencias de su equipo.

## Objetivo

Habilitar el flujo completo de asistencia por QR: generación del QR por sesión, registro de asistencia escaneando ese QR, y búsqueda de asistencias con una matriz de autorización estricta.

## Alcance

- Nueva tabla `attendances` con los tres índices indicados y regla de no duplicados (UNIQUE compuesto).
- Tres endpoints bajo `/api/v1/attendance`.
- Matriz de autorización del endpoint de búsqueda (owner vs. corredor).
- Manejo de errores estandarizado (400/401/403/404) consistente con `APIError`.

## No alcance

- CRUD / modelo completo de `training_sessions` (la tabla llegará en un change futuro; por eso `training_session_id` se modela como FK opaca, ver design).
- Edición, borrado o soft-delete de asistencias.
- Estadísticas, reportes o exportación de asistencias.
- Push notifications al registrar asistencia.

## Métrica de éxito

- `go test ./...` en verde con coverage global por encima del umbral de CI (≥80%).
- El POST es idempotente: registros duplicados del mismo `(team_id, training_session_id, user_id)` no crean filas nuevas.
- La matriz de autorización responde 403 para accesos no permitidos y 404 para entidades inexistentes.

## What Changes

- Nueva tabla `attendances` (id, team_id, training_session_id, user_id, created_at, updated_at) con:
  - `UNIQUE (team_id, training_session_id, user_id)`
  - `INDEX (team_id, training_session_id)`
  - `INDEX (user_id, team_id)`
- Modelo GORM `Attendance` en `domains/dbs/`; `training_session_id` como FK opaca (INT > 0, sin constraint a `training_sessions` por ahora).
- Tres endpoints nuevos (todos detrás del `AuthMiddleware`):
  - `GET /api/v1/attendance/qr?team_id={id}&training_session_id={id}` → devuelve QR determinista en base64 + URL codificada.
  - `POST /api/v1/attendance/team/{team_id}/session/{training_session_id}` → registra asistencia (201 primera vez, 200 si ya existía).
  - `GET /api/v1/attendance/search?team_id=&training_session_id=&user_id=` → busca con matriz de autorización.
- Nueva capability `attendance-qr` con sus requirements: generación determinista de QR, registro idempotente vía UNIQUE (sin SELECT previo), matriz de autorización de búsqueda y validación de parámetros (> 0).
- Nueva dependencia: librería de generación de QR (`github.com/skip2/go-qrcode`) con encoding PNG determinista.
- Documentación Swagger con las anotaciones de los controllers nuevos.

## Capabilities

### New Capabilities

- `attendance-qr`: registro de asistencias por QR y búsqueda con matriz de autorización (generación de QR determinista, alta idempotente, búsqueda filtrada por owner/corredor).

### Modified Capabilities

- (ninguna — no se cambian requirements de capabilities existentes)

## Impact

**Código nuevo:**
- `cmd/api/domains/dbs/attendance.go` — modelo GORM de la tabla `attendances`.
- `cmd/api/domains/attendance/` — DTOs de request/response del dominio.
- `cmd/api/daos/attendance_dao.go` — acceso a datos (insert atómico, búsqueda con filtros, chequeo de ownership).
- `cmd/api/services/attendance_service.go` — lógica de negocio (matriz de autorización, construcción de URL, validaciones).
- `cmd/api/controllers/attendance_controller.go` — handlers HTTP de los tres endpoints.
- `cmd/api/app/url_mappings.go` + `app.go` — wiring de rutas y dependencias.

**Código modificado:**
- `go.mod` / `go.sum` — nueva dependencia de QR.
- `cmd/api/docs/` — regeneración de Swagger (no se edita a mano).

**Decisiones de diseño relevantes:**
- `training_session_id` como FK opaca: no existe `training_sessions` aún; el change futuro que la agregue incorporará la constraint.
- Autorización de búsqueda sobre la membresía real del sistema: owner = `teams.owner_id`, pertenencia = `team_users.user_id`.