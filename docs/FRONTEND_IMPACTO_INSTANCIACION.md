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

`session_id` (ID bare de catálogo) ya no existe. Ahora el día embebe la copia congelada asignada. `session_instance: null` cuando el día no tiene instancia (`kind=rest`/`other`); con `kind=cancelled` la instancia **sigue embebida** (el código conserva el `session_instance_id` original al cancelar, ver abajo).

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

`exercises` es un array (puede venir vacío, pero nunca `null`), y cada elemento es la `ExerciseInstance` congelada + el `role`/`repeat_count`/`rest_minutes` del vínculo (`session_exercise_instances`). Desde el change `instancia-referencia-catalogo` el objeto embebido trae además `session_id`/`exercise_id` (origen de catálogo, puede ser `null`) — ver §6, no requiere regenerar este ejemplo.

**IMPORTANTE — Rompe el contrato actual, no es aditivo:** el frontend (que ya no puede resolver contenido por `session_id`) debe renderizar el evento a partir del objeto embebido.

Notas:

- **Días con `kind=cancelled`** (solo alcanzable desde `training`) **conservan** el `session_instance_id` original como contexto — `session_instance` NO es `null` ahí: sigue embebida para que el alumno vea qué sesión se canceló.
- Los horarios `presencial_time_from`/`presencial_time_to` viajan `"HH:MM"` (UTC) como siempre — sin cambio. `stamp` con `force=true` reemplaza el contenido instanciando de nuevo y borrando la instancia vieja (salvo feedback, D10).

## 4. Guard de día cerrado: `422` con la lista de fechas

La regla de "día cerrado" es la misma de siempre (`calendario-asignacion-grupos` D13): `date` pasada; hoy presencial después de `presencial_time_from`; hoy async. Cambia su uso: ya no clonea, **bloquea la escritura**.

Aplica a los 5 endpoints: `PUT /groups/{id}/calendar/{date}`, `DELETE`, `stamp`, `bulk`, `bulk-clear`, y a `shift` sobre las filas afectadas. En bulk es **todo-o-nada**: se validan todas las fechas antes de escribir y si alguna está cerrada se rechaza el lote completo con las fechas listadas:

```json
HTTP 422
{"message": "el día de calendario está cerrado: 2026-09-19, 2026-09-22"}
```

El mensaje del 422 **siempre incluye la fecha** también en el caso individual: `{"message": "el día de calendario está cerrado: 2026-09-19"}` — lo que cambia en lote es que el mensaje lista **todas** las fechas conflictivas, no una sola.

**La única operación exenta: la transición a `kind=cancelled`** (desde `training`). Sigue permitida incluso sobre un día ya cerrado — cancelar no repuntea nada, solo marca `cancelled_reason` y conserva la instancia. Reasignar, borrar o correr un día cerrado: prohibido, siempre.

## 5. Sobre `docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md` (repo frontend)

El **§5** de ese documento (clonado por divergencia al editar sesión) **queda obsoleto** — describe el mecanismo eliminado. Coordina su actualización/eliminación desde el repo de `paceron-frontend`; este repo no lo tocará. Su **Gap 7** (referencias al catálogo de origen + PUT sin `session_id` conservando instancia) **queda cerrado** por el change `instancia-referencia-catalogo` — ver §6 abajo, aditivo.

## 6. Cambio aditivo: origen de catálogo + `session_id` opcional (`instancia-referencia-catalogo`)

Cerrado el Gap 7. **Todo lo de esta sección es aditivo — clientes viejos no tienen acción obligatoria** (ningún campo existente cambió de nombre/semántica; nada que antes funcionaba deja de hacerlo).

1. **`session_instance.session_id` y `session_instance.exercises[].exercise_id`** (`int64 | null`): referencia al `Session`/`Exercise` del catálogo de origen. Poblada en toda instancia creada desde ahora; **`null` en instancias anteriores** al change (legado — no hay backfill). Informativa: no es FK, puede apuntar a una fila de catálogo soft-borrada (deja de figurar en `GET /sessions`/`GET /exercises` pero el ID sigue resolviendo por `GET /sessions/{id}` mientras no se borre físicamente).

   ```json
   "session_instance": {
     "id": 123,
     "session_id": 77,
     "name": "Fartlek 5K",
     "exercises": [ { "id": 456, "exercise_id": 501, "name": "Trote", "…": "…" } ]
   }
   ```

2. **`PUT /groups/{id}/calendar/{date}` con `kind=training` ya no exige `session_id` si el día ya tiene instancia**: omitirlo conserva la instancia tal cual (no reinstancia, no borra nada) — sirve para editar `is_presencial`/horarios/otros campos del día sin tocar el contenido. Si el día **no** tiene instancia y se omite, `422` (combinación de campos, como antes).
3. **`bulk` mismo criterio por fecha**: `kind=training` sin `session_id` conserva la instancia de cada fecha. Si **alguna** fecha no tiene instancia, se rechaza el lote entero (`422`, all-or-nothing) listando las fechas:

   ```json
   HTTP 422
   {"message": "los días indicados no tienen una sesión instanciada que conservar: 2026-09-22, 2026-09-23"}
   ```

4. **`stamp` invariable**: los días de plan de entrenamiento referencian el catálogo por definición, `session_id` sigue siendo requerido ahí.

## 7. Cambio aditivo: `exclude_dates` en stamp (`stamp-exclude-dates`)

Cerrado el Gap 8 ("evitar pisar selectivo" en el preview de estampado). **Aditivo — omitir el campo mantiene el comportamiento actual exacto.**

`POST /groups/{id}/calendar/stamp` acepta `exclude_dates` opcional: array de fechas `"YYYY-MM-DD"` del rango objetivo que se saltan por completo.

```json
{ "plan_id": 5, "start_date": "2026-10-05", "force": true, "exclude_dates": ["2026-10-07", "2026-10-09"] }
```

- Cada fecha excluida **no se toca**: si el día ya tenía fila/instancia, quedan intactas (mismo `session_instance_id`); si estaba vacío, sigue vacío.
- No cuenta para el `409` de conflictos ni para el `422` de día cerrado — se ignora antes de evaluar cualquier guarda. `force` sigue aplicando igual sobre las fechas **no** excluidas.
- La respuesta (`201`, array de días estampados) no incluye las fechas excluidas.
- Casos borde: fecha excluida fuera del rango del plan → se ignora silenciosamente; formato inválido en el array (`"10/07/2026"`) → `422 "exclude_dates debe tener formato YYYY-MM-DD"` sin escribir nada; rango totalmente excluido → `201` con `[]` (no es error).
