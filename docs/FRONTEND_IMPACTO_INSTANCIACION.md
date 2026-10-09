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
- La respuesta (`201`, wrapper `{days, same_team_warnings?}` — ver §8.2) no incluye las fechas excluidas.
- Casos borde: fecha excluida fuera del rango del plan → se ignora silenciosamente; formato inválido en el array (`"10/07/2026"`) → `422 "exclude_dates debe tener formato YYYY-MM-DD"` sin escribir nada; rango totalmente excluido → `201` con `{"days": []}` (no es error).

## 8. Colisión presencial, banners nuevos y calendario agregado (`colisiones-presenciales-y-calendario-agregado`)

Cambios de contrato **confirmados en el código final** de la rama `feature/colisiones-presenciales-y-calendario-agregado`. Referencias: `cmd/api/domains/calendar/*.go`, `cmd/api/controllers/calendar_controller.go`, `cmd/api/services/calendar_service.go`. Spec: `openspec/changes/colisiones-presenciales-y-calendario-agregado/`; doc de dominio: `docs/CATALOGO_Y_CALENDARIO.md` §8.7/§8.8.

Hay **2 cambios breaking** (§8.1 y §8.2) y el resto es aditivo/nuevo.

### 8.1 BREAKING — `GET /users/{id}/next-session`: shape nuevo, nunca 204

El shape anterior (una sola sesión con `session_instance` embebida, `204` si no había) **ya no existe**. Ahora siempre responde `200` con `{next_cancelled, next_training}`, cada uno el más próximo de su kind entre los grupos con membresía activa del usuario, independientes y nullable (`null` cuando no hay próxima de ese kind — ya no hay `204`):

```json
{
  "next_cancelled": {"group_id": 1, "group_name": "Maratón A", "date": "2026-09-25", "session_name": "Trote suave"},
  "next_training": {
    "group_id": 2, "group_name": "Fondo B", "date": "2026-09-26", "session_name": "Fartlek 5K",
    "is_presencial": true,
    "presencial_time_from": "18:00", "presencial_time_to": "19:00",
    "presencial_location": {"lat": -31.4, "lng": -64.2, "label": "Parque Sarmiento"}
  }
}
```

- `next_cancelled`: plano `{group_id, group_name, date, session_name}`.
- `next_training`: agrega `is_presencial`, `presencial_time_from`/`to` (`"HH:MM"`, `null` si async) y `presencial_location` (`{lat, lng, label?}`, `null` si async).
- `session_name` puede ser `null` (instancia sin nombre resoluble).
- Filtro "hoy cuenta": hoy presencial **ya arrancado** no aparece; hoy presencial por arrancar y hoy async sí.

Acción frontend: reemplazar el parseo del shape viejo (objeto único con `session_instance`) por los dos campos; eliminar cualquier manejo de `204` en este endpoint.

### 8.2 BREAKING — `stamp`/`bulk`/`shift`: wrapper `{days, same_team_warnings}` en vez de array crudo

Las 3 escrituras por lote devuelven ahora un objeto, no un array:

```json
{ "days": [ { "id": 1, "group_id": 2, "…": "…" } ], "same_team_warnings": [ { "…": "…" } ] }
```

- `days` siempre viaja (vacío solo en el caso "rango totalmente excluido" del stamp, `201` con `{"days": []}`).
- `same_team_warnings` es opcional (`omitempty`): solo aparece si la escritura guardó días presenciales que superponen con otros grupos **del mismo equipo** (ver §8.4). Los items son `PresencialConflict` (shape en §8.4).
- Códigos de éxito sin cambios: stamp `201`, bulk/shift `200`.

Acción frontend: dejar de iterar la respuesta como array; leer `.days`.

`PUT /groups/{id}/calendar/{date}` **no** cambia de shape: sigue devolviendo `CalendarDayResponse` plano, con `same_team_warnings` como campo extra opcional.

### 8.3 NUEVO — `GET /users/{id}/next-presencial-session` (banner del entrenador)

La próxima sesión `training`+`presencial` entre **todos** los grupos que administra el usuario (owner de sus equipos), la primera cronológicamente sin importar el equipo, mismo filtro "hoy cuenta" que next-session. `200` con:

```json
{
  "group_id": 2, "group_name": "Fondo B", "team_id": 1, "team_name": "Equipo A",
  "date": "2026-09-26", "session_name": "Fartlek 5K",
  "presencial_time_from": "18:00", "presencial_time_to": "19:00",
  "presencial_location": {"lat": -31.4, "lng": -64.2, "label": "Parque Sarmiento"}
}
```

o `204` si no hay ninguna (incluido el caso "no administra ningún grupo"). Guard `id == callerID` (`403` por id ajeno), como los demás endpoints de usuario.

### 8.4 409 de colisión presencial + `same_team_warnings`

Al escribir (PUT/stamp/bulk/shift), si lo que se guarda queda `training`+`presencial` y **superpone** (mismo día, overlap medio-abierto: terminar 09:00 y arrancar 09:00 **no** colisiona) con un día presencial de otro grupo del mismo entrenador:

- **Grupo de OTRO equipo → `409`**, escritura rechazada (all-or-nothing en lote; en shift rollback completo). Body JSON dedicado (no el string plano del resto de los errores):

  ```json
  HTTP 409
  {
    "message": "colisión presencial con otro equipo",
    "conflicts": [
      {"group_id": 3, "group_name": "Maratón B", "team_id": 2, "team_name": "Equipo B",
       "date": "2026-10-01", "presencial_time_from": "09:00", "presencial_time_to": "10:00"}
    ]
  }
  ```

  `conflicts` lista todos los colisionantes. `force=true` del stamp **no** la bypasea. Días `cancelled` (y cualquier no training+presencial) quedan fuera de la detección en ambos lados.

- **Grupo del MISMO equipo → warning no bloqueante**: la escritura tiene éxito y la respuesta trae `same_team_warnings` (en PUT, campo extra del `CalendarDayResponse`; en stamp/bulk/shift, campo del wrapper §8.2). Mismo shape que `conflicts` arriba.

Acción frontend: mostrar los conflictos del 409 (grupo, fecha, horario, equipo) para que el entrenador elija otro horario, y los warnings como aviso no bloqueante post-guardado.

### 8.5 NUEVOS — `member-calendar` y `administered-calendar` (calendario agregado)

- **`GET /users/{id}/member-calendar?from=&to=`**: días de todos los grupos con membresía activa del usuario, ordenados por fecha. `200` array (vacío si no hay). `from`/`to` obligatorios (`400`), `from <= to` (`400`), guard `id == callerID` (`403`).
- **`GET /users/{id}/administered-calendar?from=&to=`**: días de todos los grupos que administra el usuario (owner de los equipos), mismo shape y validaciones.

Item (ambos): `AggregateCalendarDayResponse` = los campos de `CalendarDayResponse` (con `session_instance` embebido, §3) + `group_id`/`group_name`/`team_id`/`team_name` embebidos planos:

```json
{
  "id": 1, "group_id": 2, "group_name": "Fondo B", "team_id": 1, "team_name": "Equipo A",
  "date": "2026-09-26", "kind": "training",
  "session_instance": { "…": "… (§3)" },
  "is_presencial": true,
  "presencial_time_from": "18:00", "presencial_time_to": "19:00",
  "presencial_location": {"lat": -31.4, "lng": -64.2, "label": "Parque Sarmiento"},
  "presencial_collision": {"type": "cross_team", "conflicts": [ { "…": "… (§8.4)" } ]}
}
```

**Cómo detectar colisiones viejas en `administered-calendar`:** los días guardados **antes** de que existiera el guard de colisión aparecen marcados igual que los nuevos — `presencial_collision` se calcula sobre los datos actuales del calendario, no sobre cuándo se guardó cada fila. Reglas del marcado:

- Solo días `training`+`presencial` participan; un día `cancelled` presencial nunca trae ni genera colisión.
- `presencial_collision` presente sii el día superpone con otro día presencial de **otro grupo administrado**. `type`: `"cross_team"` si algún colisionante es de otro equipo (gana si hay de ambos), `"same_team"` si todos son del mismo. `conflicts` lista todos los colisionantes (el día colisionante aparece marcado también en su propio item).
- `presencial_collision` ausente = sin colisión. `member-calendar` **nunca** lo trae (es exclusivo de la vista del entrenador).
- El backend solo **marca** — reprogramar o cancelar uno de los dos días es acción manual del entrenador.

## 9. NUEVOS — historial de feedback (Gap 13, change `workout-feedback-history`)

Cierre del Gap 13 (`BACKEND_API_GAPS.md`): la pestaña "Historial" ya tiene endpoints. **Todo aditivo** — `GET /workout-feedback/search` sigue vivo e intacto (decisión: no se depreca); ningún shape existente cambió. Referencia completa: `docs/CATALOGO_Y_CALENDARIO.md` §8.9; shapes acá verificados contra `cmd/api/domains/workoutfeedback/workout_feedback_history.go` (DTOs con tags json), controller y service.

- **`GET /users/{id}/workout-feedback-history`** (corredor, self-only: `id` == token, `403` si no).
- **`GET /users/{id}/administered-workout-feedback-history?team_id=`** (entrenador: `id` == token, `team_id` obligatorio `400`, equipo inexistente `404`, no-owner `403`). Extra: `athlete_user_id` para filtrar a un atleta puntual.

Iteración de validaciones 400 (`apierror.APIError {status_code, code, message}`): `group_id` sin `team_id`; `date_from`/`date_to` pareados (uno solo → 400), `from > to` → 400 (iguales = 1 día, formato `YYYY-MM-DD`); `page` ≥ 1; `page_size` 1..100; `sort` ∈ {`feedback_date`, `set_number`, `exercise_name`}; `order` ∈ {`asc`, `desc`}.

Response `200 {items, total, page, page_size, available_athletes, available_exercises}` — ítem (todos los campos siempre presentes, los nullable van `null`, no hay omitempty):

```json
{
  "items": [
    {
      "id": 10,
      "athlete_user_id": 5,
      "athlete_name": "Ana Gómez",
      "team_id": 1,
      "team_name": "Equipo A",
      "group_id": 2,
      "group_name": "Fondo B",
      "date": "2026-09-20",
      "session_name": "Fartlek 5K",
      "session_instance_id": 77,
      "exercise_id": 456,
      "exercise_name": "Trote",
      "catalog_exercise_id": 501,
      "set_number": 2,
      "completion_status": "completed",
      "duration_ms": 60000,
      "active_duration_ms": 58000,
      "distance_meters": 1200.5,
      "started_at": "2026-09-20T18:00:00Z",
      "ended_at": "2026-09-20T18:10:00Z"
    }
  ],
  "total": 25,
  "page": 2,
  "page_size": 10,
  "available_athletes": [{"id": 5, "name": "Ana Gómez"}],
  "available_exercises": [{"id": 456, "name": "Trote"}]
}
```

Decisiones que el frontend necesita conocer:

- **`exercise_id` es id de instancia** (`assigned_exercise_id`), NO de catálogo. Para agrupar por ejercicio a través del tiempo usá `catalog_exercise_id` (nullable: `null` en instancias previas a la instanciación o si el feedback es huérfano sin fila de instancia).
- **`session_instance_id`** (siempre presente, > 0): es `assigned_session_id`, el id de la instancia de sesión — lo que hay que mandar a `GET /session-instances/:id/feedback` (junto al `athlete_user_id` del ítem) para la pantalla de revisión. Si la instancia fue borrada físicamente (huérfano), el id queda expuesto pero esa pantalla responderá 404.
- **Filtro `exercise_id` matchea por familia de catálogo:** el valor a mandar es el `id` de `available_exercises` (que es de catálogo): matchea todas las instancias de ese ejercicio de catálogo, con fallback a id de instancia propio para instancias legado sin origen. Los `id` del pool son directamente usables en el filtro; los `exercise_id` de los ítems siguen siendo de instancia (por fila).
- **Huérfanos se conservan en `items`** (no desaparecen del historial): `group_id`/`group_name` `null` = día de calendario borrado o instancia nunca asignada a un día; `team_id`/`team_name` `null` = feedback registrado sin equipo. `session_name`/`exercise_name` también pueden ser `null` si la instancia fue borrada. Sin campo extra de razón — los nulls lo comunican.
- **`sort=feedback_date` ordena por `session_date`** (la fecha del entrenamiento, no el timestamp de carga).
- **Pools ignoran de segundo nivel:** `available_athletes`/`available_exercises` ({id, name}) son DISTINCT sobre los matcheos del primer nivel solo (equipo/grupo/fechas + scope de autorización); `exercise_id`, `set_number` y `athlete_user_id` NO los recortan — así al elegir otro atleta/ejercicio no se achican las opciones. `available_exercises` va **dedupeado por familia**: un ítem por ejercicio de catálogo (`catalog_exercise_id`, fallback al id de instancia en instancias legado), no por instancia.
- `total` = matcheos de TODOS los filtros sin paginar; paginación `page`/`page_size` defaults 1/20.
- Sin agregados/sumarización server-side (por período/ejercicio): si el tab los necesita, por ahora se calculan client-side sobre `items`.

## 10. NUEVO — detalle standalone de instancia (Gap 14, change `session-instance-detail`)

`GET /api/v1/session-instances/{id}`: la instancia completa (el mismo objeto que el calendario embebe como `session_instance`) a partir de solo el id — el complemento del `session_instance_id` del historial (§9). Todo aditivo, nada de lo existente cambia.

```json
{
  "id": 88,
  "session_id": 12,
  "name": "Series de velocidad",
  "description": "...",
  "created_at": "2026-09-20T15:00:00Z",
  "exercises": [
    {
      "id": 501, "exercise_id": 44, "name": "Series 400m", "kind": "training",
      "description": null, "intensity": null, "minutes": null, "distance_m": null,
      "speed_kph": null, "muscle_group": null, "video_url": null,
      "role": "principal", "repeat_count": 4, "rest_minutes": 2
    }
  ]
}
```

- **Uso:** historial (§9) trae `session_instance_id` → este endpoint trae el objeto completo para la pantalla de revisión.
- **Códigos:** `404` instancia inexistente; `403` si el caller no tiene vínculo con ella. Acceso si: hay un día de calendario con esta instancia y el caller es miembro activo del grupo u owner del equipo; **o** hay un feedback activo sobre la instancia del caller (atleta/reportante/owner del equipo). Nada más.
- Los ejercicios vienen congelados por instancia (id de instancia en `id`; `exercise_id` = origen catálogo, nullable). `GET /session-instances/:id/feedback` y `/:id/runner` siguen intactos.

## 11. NUEVO — gateway WebSocket para sesiones (Gap 18, change `ws-gateway-sesiones`)

Contrato frontend del gateway WS. Referencia completa: **`docs/REALTIME_WS.md`** (shapes acá verificados contra `cmd/api/realtime/*.go` y `cmd/api/app/realtime.go`). Todo nuevo — nada de la API HTTP existente cambia.

> **NOTA — el endpoint WS NO está en el Swagger** (por diseño: el handshake WS no es OpenAPI-representable). Este doc y `REALTIME_WS.md` son su único contrato.

**Conexión:** `wss://host/api/v1/ws?token=<access_token>` — mismo access token JWT del resto de la API, en query param (no hay header `Authorization` en el handshake). Falta/inválido/expirado → `401` JSON `apierror.APIError` antes del upgrade (`code` ∈ {`unauthorized`, `token_expired`}). Clientes nativos sin header `Origin` pasan; browser con origin fuera de `CORS_ALLOWED_ORIGINS` → upgrade rechazado.

**Canales:** el único patrón es `session:{id}` con `{id}` = **session instance id** (el de `assigned_session_id` en feedback / `session_instance.id` en calendario — la copia congelada, no el id de catálogo). Suscribirse a una instancia que el usuario puede ver (misma regla dual de `GET /session-instances/{id}`) responde `{"type":"subscribed","channel":"session:123"}`; canal ajeno/desconocido responde `error` y la conexión sigue viva. Tope 20 canales por conexión; resubscribe idempotente. El ID va en decimal canónico (`session:7`, nunca `session:07`).

**Eventos:**

- `{"type":"presence","from":12,"payload":{...}}` / `{"type":"control","from":12,"payload":{...}}` — reenvíos de otros usuarios suscriptos al mismo canal (emisor excluido; `payload` opaco pero **debe ser un objeto JSON** al enviarlo — `null`/arreglo responden `error`). El destino se resuelve del `channel` del frame o de la única suscripción activa.
- `{"type":"update:set_event","channel":"session:<id>","data":{...}}` — server-originado: alguien creó feedback de esa sesión. **`data` es el MISMO body HTTP 201 de `POST /workout-feedback`** (`{message: "feedback registrado", data: {...WorkoutFeedbackResponse con athlete_user_id}}`) — reutilizar el normalizador HTTP. El frame incluye `channel` (el cliente rutea por `msg.channel` igual que los relayeados). Solo `Create` emite; nadie suscripto = evento no existe (no hay replay ni cola: lo que se perdió offline se recupera por fetch).

**Errores sin cortar conexión:** cualquier rechazo puntual (canal ajeno, canal desconocido, JSON inválido, `type` desconocido, tope de canales) llega como `{"type":"error","message":"..."}` y la conexión queda abierta y utilizable. No hacer retry-desconectar sobre `error`.

**Heartbeat JSON:** mandar `{"type":"ping"}` cada 20-30 s → responde `{"type":"pong"}`. Cualquier mensaje refresca el deadline de 45 s — sin tráfico en 45 s el servidor corta. Frames del cliente > 4 KB cortan la conexión (única excepción).

**Reconexión:** los deploys cortan las conexiones — reconectar con backoff y re-suscribirse; las suscripciones no sobreviven a la desconexión. El servidor también puede cortar por **overflow sostenido** del buffer de salida (cliente que no drena su cola): es un caso más de reconexión, no de error.

## 12. Sesión interrumpida y eventos en vivo (Gaps 19/23/25/26/27/28, change `sesion-interrumpida-y-eventos-en-vivo`)

Cambios de contrato **confirmados en el código final** de la rama `feature/sesion-interrumpida-y-eventos-en-vivo` (HEAD `1a10595`). Referencias: `cmd/api/controllers/runner_session_controller.go`, `cmd/api/controllers/attendance_controller.go`, `cmd/api/controllers/calendar_controller.go`, `cmd/api/domains/instance/instance_response.go`, `cmd/api/domains/user/search_response.go`, `cmd/api/realtime/*.go`, `cmd/api/daos/group_user_dao.go`. Doc de dominio: `docs/CATALOGO_Y_CALENDARIO.md` §8.11; contrato WS completo: `docs/REALTIME_WS.md` §6–§7.

**Marco general:** salvo Gap 25 (fix de elegibilidad, sin shape nuevo), todo es **aditivo o contrato nuevo** — `interrupted`, el gate 409, los 3 campos del detalle y los 2 eventos WS no pisan ningún shape existente. Nada de lo que hoy funciona cambia de significado.

### 12.1 Gap 19 — `PATCH /session-instances/:id/runner` con `status="interrupted"`

Nuevas transiciones (estado previo en columnas propias de runner session): `wip→interrupted` (`200`), `interrupted→finished` (`200`, re-setea `end_date` al now del server), `interrupted→interrupted` idempotente (`200`). `finished` segue siendo terminal: `finished→interrupted` → **`400`** (msg "no se puede interrumpir una sesión ya finalizada"), status inválido → `400`, fila inexistente → `404`.

```json
PATCH /api/v1/session-instances/88/runner
{"status": "interrupted"}
HTTP 200
{"message": "sesión marcada como interrumpida", "data": {"id": 1, "session_instance_id": 88, "athlete_user_id": 5, "status": "interrupted", "start_date": "2026-10-01T18:00:00Z", "end_date": null}}
```

El shape de la fila (`RunnerSessionResponse`) no cambió — solo se agregó el valor del enum de `status`.

### 12.2 Gap 26 — apertura/cierre presencial: gate 409 y detalle con 3 campos

**Gate de escritura del corredor no-owner** sobre día presencial (owner del team del grupo exento): día cerrado → `409 {"status_code":409,"code":"session_closed","message":...}`; día presencial nunca abierto → `409 {"status_code":409,"code":"session_not_opened","message":...}`. Orden de errores: `404` instancia → `403` atleta ajeno → `409` gate. Días no presenciales sin gate. Acción frontend: refetch del detalle (o consumo de `update:session_state`) para distinguir los dos slugs.

**Detalle de instancia** (`GET /session-instances/{id}`, §10): `session_instance` suma 3 campos `omitempty` — solo presentes en el path de detalle, solo para día presencial:

```json
"session_instance": {
  "id": 123, "name": "Fartlek 5K", "…": "…",
  "presencial_open": true,
  "opened_at": "2026-10-01T18:02:11Z",
  "closed_at": null
}
```

Cerrada: `presencial_open: false` + `closed_at` poblado (el cierre es final, no hay reopen). Instancia huérfana, día no presencial o paths de calendario (GetRange/NextSession/member-calendar): los 3 ausentes del JSON. El objeto standalone del detalle los trae igual al nivel raíz.

### 12.3 Gap 23 — `photo_url` en search y batch lookup

Aditivo: `SearchResultItem` trae `photo_url` nullable **sin `omitempty`** (siempre presente en el JSON; `null` sin foto o foto nunca actualizada):

```json
{"user_id": 5, "name": "Ana Gómez", "email": null, "photo_url": "https://cdn/…?v=1690000000"}
```

Con foto: URL firmada de `buildMediaURL(key, photo_updated_at)` (`?v=<unix de photo_updated_at>`); `null` cuando `photo_key` es `NULL`. Igual en `Search` y `BatchLookup`.

### 12.4 Gap 25 — roster/asistencia: elegibilidad por fecha

Fix de comportamiento, sin shape nuevo: la membresía activa se evalúa por **fecha de calendario** (`date_start::date <= fecha de la sesión` y `date_end::date >=fecha de la sesión`). Corredor con membresía que empieza el mismo día (a cualquier hora) ya aparece en `FindGroupRosterWithAttendance` y es elegible para attendance. Si el frontend filtraba roster por membresía propia, ya no hace falta compensarlo.

### 12.5 Gap 27 — relay dirigido por `to`

Nuevo mandato en presence/control: indicar el destino con `"to": <userID entero>` **dentro del payload** → entrega **solo a las conexiones de ese user** en el canal (todas, no una específica). Semántica completa:

- `to` entero en `[1, MaxInt64]` → dirigido. `to` = propio userID → el emisor **sí recibe con eco** (única forma de auto-eco en presence/control).
- `to` ausente o `"all"` → actual: todos los suscriptos menos el emisor.
- Otro tipo (`"to":"5"`, decimal `5.5`, overflow `> MaxInt64`, arreglo) → broadcast normal.
- El frame entregado lleva el payload completo, `to` incluido adentro (el backend no lo muta ni lo remueve) — el receptor puede ver a quién apuntaba.
- Nadie tiene ese userID suscripto → nadie lo recibe, la conexión del emisor sigue viva.

Vía HTTP no cambia nada: es semántica de entrega del WS.

### 12.6 Gap 28 — evento `update:attendance_event`

Server-originado, al canal `session:{training_session_id}` (que es el session instance id de la fila de attendance). Frame: `{"type":"update:attendance_event","channel":"session:88","data":{"user_id":5,"status":"attended","source":"qr","registered_at":"2026-10-01T18:05:00Z","attendance_id":42}}` — `data` es la fila del roster afectada igual que en la grilla (sin name/email; días 200 idempotente no emiten; cache offline = refetch del roster). Ver detalle completo de alcance por operador en `docs/REALTIME_WS.md` §6.

Eventos companion para el día: apertura/`finished` del entrenador → `update:session_state` con `{presencial_open, opened_at, closed_at}` (ver §6 de REALTIME_WS). El `interrupted` del entrenador no emite (no cierra).

## 13. Tiers/fees/pagos y mensajería de sesión (Gaps 5/15/16/17/27, change `permisos-tier-fees-y-mensajeria-sesion`)

Cambios de contrato **confirmados en el código final** de la rama `feature/tiers-fees-pagos-y-mensajeria` (HEAD `399dd1b`). Referencias: `cmd/api/controllers/tier_permission_controller.go`, `cmd/api/domains/team/team_search.go`/`team_response.go`, `cmd/api/domains/invitation/invitation_response.go`, `cmd/api/domains/sessionmessage/*.go`, `cmd/api/services/session_message_service.go`, `cmd/api/controllers/payment_controller.go`, `cmd/api/realtime/notifier.go`. Shapes verificados contra tags json de los DTOs.

**Marco general: aditivo.** Gap 16 solo corrige códigos de error de `CreatePreference` (el 500 genérico sigue existiendo para causas no identificables) y Gaps 15/17 agregan campos a responses existentes — ningún shape anterior pierde campos ni cambia de nombre; lo que hoy funciona sigue funcionando. Gap 5 y Gap 27 son endpoints/eventos nuevos.

### 13.1 Gap 5 — `GET /tiers/{id}/permissions`

Cualquier usuario autenticado (sin ownership). `404` si el tier no existe (message "tier no encontrado"); `500` ante error interno. `200` si existe — el shape:

```json
{"permissions": [{"permission_id": 3, "permission_name": "ver_asistencias"}]}
```

- Orden por `permission_id` ASC (siempre); sin permisos asignados → `{"permissions": []}` (array vacío, nunca `null`).
- Un permiso **soft-deleted** que aún tiene asignación activa (la asignación no se limpia) **se omite** del listado: el array expone solo permisos activos.

### 13.2 Gaps 15+17 — `membership_fee` y `can_receive_payments` en team/invitaciones

Campos **aditivos** (aparecen a la cola del objeto, sin `omitempty` — viajan siempre, `membership_fee` `0` = gratis):

- **`TeamSearchResult`** (`GET /teams/search`, también {teams, has_more}): `membership_fee` (float) + `can_receive_payments` (bool).
- **`InvitationResponse`** (listado/detalle de invitaciones): ambos. El fee es el **vigente del team al consultar** (no congelado al momento de invitar — puede diferir del fee que la invitación tenía al crearse).
- **`TeamResponse`**: `can_receive_payments` (bool). Fee ya existía en este shape. Derivado en **todas sus rutas**: GetAll (batch), GetByID, Create, Update, UpdateAddress.

**Criterio de `can_receive_payments`:** owner del team con `seller_connections` del app actual (`client_id` = config OAuth MP) en estado `authorized` **y** `public_key != ""` (el desconectado del pseudo-shape o conectado sin public_key da `false`). Fallo del lookup (error de DB) → `false` (nunca rompe la respuesta).

El frontend usa: cards de search que ocultan el botón de suscripción si `can_receive_payments = false`; detalle/invitación muestran la mensualidad real.

### 13.3 Gap 16 — códigos de error reales en `CreatePreference`

`POST /recommenders/pagos` (flujo team_subscription) hoy respondía 500 para todo. Ahora `mapPreferenceError` mapea (body siempre `apierror.APIError {status_code, code, message}`):

| Caso | HTTP | `code` | `message` |
|---|---|---|---|
| Owner desconectado de MP | `409` | `SELLER_NOT_CONNECTED` | "el entrenador debe conectar su cuenta de Mercado Pago" |
| Owner con conexión sin `public_key` (conectado antes de que se guardara) | `409` | `SELLER_NOT_CONNECTED` | "el entrenador debe **reconectar** su cuenta de Mercado Pago" |
| Cuota de suscripción inexistente / no es de equipo | `404` | `Not found` | message del sentinel ("cuota no encontrada" / "la cuota no pertenece a un equipo") |
| Equipo inexistente | `404` | `Not found` | "equipo no encontrado" |
| Request inválido (`installment_id` faltante para team_subscription, etc.) | `400` | `Bad request` | message del sentinel |
| Cualquier otro error (upstream MP, DAO, descifrado…) | `500` | `Internal server error` | "Error al crear la preferencia" (genérico, **sin exponer el error interno**) |

- El `409 SELLER_NOT_CONNECTED` **sustituye** el 500 previo para ese caso; el 500 sigue existiendo para causas no identificables — no es breaking de shapes.
- El chequeo de resolución del split corre **antes** de crear la fila de pago — un error tipado no deja fila huérfana.
- Acción frontend: el 409 mostrar el modal "conectá tu cuenta" (y si el message dice "reconectar", pedirla de vuelta y no solo conectar — la conexión previa quedó sin datos necesarios).

### 13.4 Gap 27 — mensajería de sesión (chat en vivo) + aviso WS

Chat por instancia de sesión, sobre `session-instances`. Doc de dominio completo: `docs/CATALOGO_Y_CALENDARIO.md` §8.12; frame WS: `docs/REALTIME_WS.md` (nuevo evento de tabla).

- **`POST /api/v1/session-instances/{id}/messages`** — `201` con:

  ```json
  {
    "id": 42,
    "session_instance_id": 88,
    "sender_user_id": 5,
    "sender_role": "trainer",
    "type": "aviso",
    "recipient_mode": "multiple",
    "recipient_user_ids": [7, 9],
    "body": "Nos juntamos en la entrada principal",
    "reply_to_message_id": null,
    "created_at": "2026-10-09T18:05:00Z"
  }
  ```

  Todos los campos sin `omitempty` — `recipient_user_ids` va **`[]`** con `recipient_mode="all"`, `reply_to_message_id` `null` si no es respuesta a otro.
- `sender_role` **deriva el backend** (el request no lo acepta): `trainer` si el emisor es owner del team del grupo del día; `runner` en cualquier otro caso.
- **Request:** `{type, recipient_mode, recipient_user_ids?, body, reply_to_message_id?}` con bindings required en `type`/`recipient_mode`/`body`.
- **Códigos:** `404` instancia inexistente (message "sesión instancia no encontrada"); `403` sin acceso a la instancia (message "no autorizado"); `400` validaciones (ver §8.12 de CATALOGO_Y_CALENDARIO — `type` ∈ {info/aviso/alerta}, `recipient_mode` ∈ {all/multiple/direct} con `recipient_user_ids` vacío / 1 / ≥2 respectivamente; `body` no vacío tras trim, tope 2000; destinatarios deben **participar de la sesión**; reply válido: existe + misma sesión + visible para el emisor).
- **`GET /api/v1/session-instances/{id}/messages?since=<id>`** — `200`:

  ```json
  {"messages": [ { "id": 42, "session_instance_id": 88, "sender_user_id": 5, "sender_role": "trainer", "type": "aviso", "recipient_mode": "multiple", "recipient_user_ids": [7, 9], "body": "…", "reply_to_message_id": null, "created_at": "2026-10-09T18:05:00Z" } ]}
  ```

  Visibilidad (por mensaje): lo que emitió, todo `recipient_mode=all`, y los que lo tienen como destinatario. Orden por `id` ASC, sin paginación server-side. `since` es el cursor de catch-up: mensajes con `id > since`; ausente/`0` = historial completo; negativo o no numérico → `400`.
- **Aviso WS:** tras cada POST exitoso, a todos los suscriptos de `session:{id}` (sin exclusión de emisor):

  ```json
  {"type":"control:message_created","channel":"session:88","payload":{"sessionMessageId":42}}
  ```

  Payload mínimo, **sin contenido** del mensaje: es un "algo llegó" — el contenido se recupera por REST con `since` (fijar `since` al último `id` visto). Best-effort: nadie suscripto → no-op. Usar `payload.sessionMessageId` solo junto al refetch (ej. conteo de no-leídos).
- Acción frontend: al recibir el frame refetchear `?since=<último id>`, push a la lista; mantener el último `id` procesado por sesión.
