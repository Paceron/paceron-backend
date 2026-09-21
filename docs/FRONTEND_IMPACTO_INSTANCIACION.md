# Impacto en `paceron-frontend` — instanciación del calendario (`asignacion-por-instanciacion`)

Cambios de contrato **confirmados en el código final** de la rama `feature/asignacion-por-instanciacion` (no son "pueden cambiar" — ya están implementados). Referencias: `cmd/api/services/calendar_service.go`, `cmd/api/domains/instance/instance_response.go`, `cmd/api/domains/calendar/*.go`, `cmd/api/domains/session/session_request.go`, swagger generado en `cmd/api/docs/`.

Spec: `openspec/changes/asignacion-por-instanciacion/` (design.md + specs/group-calendar/spec.md).

Este repo **no modifica el frontend** — coordinar con quien mantenga `paceron-frontend`.

## 1. `PUT /sessions/{id}` ya no acepta campos de clonado

`PUT /api/v1/sessions/{id}` ya no acepta `exclude_group_ids`, `clone_name` ni `clone_description`. El `SessionRequest` quedó solo con `owner_id`, `name`, `description`, `exercises`. Si el frontend todavía los manda, **se ignoran silenciosamente** (no hay 400 ni warning).

Edición de catálogo jamás repuntea asignaciones de calendario. La UI de "mostrá qué grupos tienen esta sesión asignada antes de editar" perdió su razón de ser.

## 2. `GET /sessions/{id}/assigned-groups` eliminado, sin reemplazo

Responde 404 (ruta no registrada). No hay endpoint alternativo, y tampoco hace falta: bajo el modelo nuevo, editar una sesión no puede afectar asignaciones existentes.

## 3. `CalendarDayResponse` / `NextSessionResponse`: `session_id` → `session_instance` embebido

`session_id` (ID bare de catálogo) ya no existe. Ahora el día embebe la copia congelada asignada. `session_instance: null` cuando `kind` no es `training`.

Ejemplo real (shape de `calendar.CalendarDayResponse` + `instance.SessionInstanceResponse`):

```json
{
  "id": 1,
  "group_id": 2,
  "date": "2026-09-21",
  "kind": "training",
  "other_name": null,
  "session_instance": {
    "id": 123,
    "name": "Fartlek 5K",
    "description": null,
    "created_at": "2026-09-20T10:00:00Z",
    "exercises": [
      {
        "id": 456,
        "name": "Trote",
        "kind": "jogging",
        "description": null,
        "intensity": null,
        "minutes": 10,
        "distance_m": null,
        "speed_kph": null,
        "muscle_group": null,
        "video_url": null,
        "role": "warmup",
        "repeat_count": 1,
        "rest_minutes": 0
      }
    ]
  },
  "cancelled_reason": null,
  "is_presencial": false,
  "presencial_time_from": null,
  "presencial_time_to": null,
  "presencial_location": null,
  "source_plan_id": null,
  "created_at": "2026-09-20T10:00:00Z",
  "updated_at": "2026-09-20T10:00:00Z"
}
```

`exercises` es un array (puede venir vacío, pero nunca `null`), y cada elemento es la `ExerciseInstance` congelada + el `role`/`repeat_count`/`rest_minutes` del vínculo (`session_exercise_instances`).

**IMPORTANTE — Rompe el contrato actual, no es aditivo:** el frontend (que ya no puede resolver contenido por `session_id`) debe renderizar el evento a partir del objeto embebido.

Notas:

- **Días con `kind=cancelled`** (solo alcanzable desde `training`) **conservan** el `session_instance_id` original como contexto — `session_instance` puede no ser `null` ahí.
- Los horarios `presencial_time_from`/`presencial_time_to` viajan `"HH:MM"` (UTC) como siempre — sin cambio. `stamp` con `force=true` reemplaza el contenido instanciando de nuevo y borrando la instancia vieja (salvo feedback, D10).

## 4. Guard de día cerrado: `422` con la lista de fechas

La regla de "día cerrado" es la misma de siempre (`calendario-asignacion-grupos` D13): `date` pasada; hoy presencial después de `presencial_time_from`; hoy async. Cambia su uso: ya no clonea, **bloquea la escritura**.

Aplica a los 5 endpoints: `PUT /groups/{id}/calendar/{date}`, `DELETE`, `stamp`, `bulk`, `bulk-clear`, y a `shift` sobre las filas afectadas. En bulk es **todo-o-nada**: se validan todas las fechas antes de escribir y si alguna está cerrada se rechaza el lote completo con las fechas listadas:

```json
HTTP 422
{"message": "el día de calendario está cerrado: 2026-09-19, 2026-09-22"}
```

(O día individual: `{"message": "el día de calendario está cerrado"}` — una sola fecha no se concatena.)

**La única operación exenta: la transición a `kind=cancelled`** (desde `training`). Sigue permitida incluso sobre un día ya cerrado — cancelar no repuntea nada, solo marca `cancelled_reason` y conserva la instancia. Reasignar, borrar o correr un día cerrado: prohibido, siempre.

## 5. Sobre `docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md` (repo frontend)

El **§5** de ese documento (clonado por divergencia al editar sesión) **queda obsoleto** — describe el mecanismo eliminado. Coordina su actualización/eliminación desde el repo de `paceron-frontend`; este repo no lo tocará.
