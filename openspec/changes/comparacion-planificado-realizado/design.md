# Diseño: Comparación planificado vs realizado

## Context

Relevado en el código y en la base de testing antes de diseñar:

- **Cada día de calendario tiene su propia sesión.** Al estampar un plan, cada día
  de entrenamiento recibe una `session_instance` nueva con copias de sus
  ejercicios (`instantiateSessionsBatch`). Una instancia pertenece a un solo día.
- **Los objetivos viven en `exercise_instances`**: `minutes`, `distance_m`,
  `speed_kph`, `kind`, `intensity`. La cantidad de series planificadas es
  `session_exercise_instances.repeat_count`.
- **`workout_feedback.assigned_exercise_id` es `exercise_instances.id`**, no el id
  de `session_exercise_instances`. En la base de testing las 95 filas cumplen
  eso, y 62 coinciden además con un id de `session_exercise_instances` por
  casualidad (los dos se crean juntos).
- **`set_number` cambió de base.** Las filas sincronizadas antes del 2026-09-30
  están en base 0; desde entonces la app manda base 1.
- **Puede haber dos filas para la misma serie.** El índice único incluye
  `feedback_owner_user_id`, así que el corredor y el entrenador pueden cargar la
  misma serie por separado.
- **Hay ejercicios sin objetivo**: entrada en calor o elongación sin minutos ni
  distancia.
- **`group_calendar_days.source_plan_id` no es confiable**: `UpsertDay` y `Bulk`
  lo pisan con NULL al editar el día.
- **Dejar un grupo pone `group_users.deleted_at`**: las consultas de "miembro
  hoy" no sirven para saber si alguien era miembro en una fecha pasada.

## Goals / Non-Goals

**Goals:**
- Detalle de una sesión con planificado y realizado por serie.
- Lista de sesiones de un rango con su cumplimiento y un resumen.
- Mismas reglas de acceso que el resto de las lecturas de sesiones.

**Non-Goals:**
- Comparación por plan de entrenamiento, exportación, gráficos y persistencia del
  cálculo (ver `proposal.md`).

## Decisions

### D1. El id del endpoint de detalle es la sesión ejecutada

La tarea pedía `GET /api/activities/comparison/{planId}`. Se decidió con el
equipo que `{planId}` sea el `session_instance_id` (una sesión de un día): es lo
que el corredor ejecuta y lo que referencia el feedback. Para el rango se agrega
un endpoint de lista. Se usa el prefijo `/api/v1/` como el resto de las rutas.

### D2. DAO nuevo y de solo lectura

Las consultas viven en `activity_comparison_dao.go` en lugar de sumarse a
`WorkoutFeedbackDAOInterface`: esa interfaz ya es grande, tiene un mock escrito a
mano en varios tests, y estas consultas cruzan cinco tablas que no son solo
feedback. El acceso a la instancia se sigue chequeando con
`SessionInstanceDao.HasInstanceAccess`.

### D3. Acceso

- El atleta es el usuario autenticado, salvo que venga `athlete_user_id`.
- Detalle propio: hace falta `HasInstanceAccess(instancia, usuario)`.
- Detalle de otro atleta: el usuario tiene que ser dueño del equipo de la sesión
  (por el día o por el feedback) **y** el atleta tiene que tener acceso a la
  instancia.
- Lista de otro atleta: `team_id` es obligatorio, el usuario tiene que ser dueño
  de ese equipo y el atleta, miembro.
- Instancia inexistente: 404. Sin acceso: 403.

### D4. Cumplimiento por serie, con tope de 100%

Cada serie planificada vale lo mismo:

- **Completada con objetivo de tiempo:** `min(duración / (minutes · 60000), 1)`.
  La duración es `active_duration_ms` si es mayor que 0; si no, `duration_ms`.
- **Completada sin tiempo pero con distancia:** `min(distancia / distance_m, 1)`.
- **Completada sin objetivo:** 1.
- **Salteada o sin registro:** 0.

El tiempo tiene prioridad sobre la distancia porque los planes se escriben así
("2 series de 10 minutos"). El cumplimiento del ejercicio es el promedio de sus
`repeat_count` series, y el de la sesión, el promedio de todas las series
planificadas. Los porcentajes se redondean a 1 decimal, igual que
`buildAttendanceSummary`. Una sesión sin series planificadas da 0.

### D5. Desviación y ritmo

- `deviation_duration_ms = duración - minutes · 60000` y
  `deviation_distance_m = distancia - distance_m`. Son `null` si no hay
  objetivo de ese tipo o si la serie no se completó.
- Ritmo planificado: `3600 / speed_kph` segundos por km, si hay velocidad.
- Ritmo real: el `pace_sec_per_km` guardado si existe; si no,
  `duración_ms / distancia_m` (segundos por km), solo si la distancia es mayor
  que 0.

### D6. Normalización del feedback

Antes de comparar:

- Se ignoran las filas con `deleted_at`.
- Si un ejercicio tiene alguna fila con `set_number = 0`, todas sus filas se
  pasan a base 1 (`+1`).
- Si hay dos filas para la misma serie, gana la del propio atleta
  (`feedback_owner_user_id = athlete_user_id`); si las dos son de otros, la más
  reciente por `updated_at`.
- Una serie con estado distinto de `completed` y `skipped` cuenta como sin
  registro.
- Las series registradas por encima de `repeat_count` se devuelven con
  `extra = true`: suman a los totales reales pero no al cumplimiento.

### D7. Qué sesiones entran en la lista

- Los días `training` del rango de los grupos donde el atleta era miembro **en
  esa fecha**: `date_start::date <= fecha` y, si están, `date_end::date >= fecha`
  y `deleted_at::date >= fecha`. Los que no ejecutó entran con 0%.
- Más las instancias con feedback del atleta en el rango que no salieron por la
  regla anterior (por ejemplo, un día que se reasignó o un grupo que el atleta ya
  dejó). Se omiten si se filtra por grupo.
- `kind` deja las sesiones con al menos un ejercicio de ese tipo.
- El rango es obligatorio (`YYYY-MM-DD`), de 366 días como máximo, y no puede
  terminar después de hoy en hora argentina: el futuro no se puede comparar.

### D8. Fecha y horario de la sesión

- Fecha: la del día de calendario; si la instancia no tiene día, la menor
  `session_date` del feedback; si tampoco hay, la fecha de creación de la
  instancia.
- Horario: inicio y fin del `runner_session`; si no hay, los horarios
  presenciales del día.
- Estado: el del `runner_session` (`wip`, `finished`, `interrupted`) o `none`.

## Contrato

```
GET /api/v1/activities/comparison/:sessionInstanceId?athlete_user_id=
→ 200 SessionComparisonResponse
  { session_instance_id, athlete_user_id, date, session_name,
    team_id, team_name, group_id, group_name, is_presencial,
    started_at, ended_at, status, kinds[],
    exercises: [{ exercise_instance_id, name, role, kind, intensity,
                  has_target,
                  planned: { sets, minutes, distance_m, speed_kph, pace_sec_per_km },
                  sets: [{ set_number, status, extra, feedback_id, duration_ms,
                           distance_meters, pace_sec_per_km, rpe, compliance_pct,
                           deviation_duration_ms, deviation_distance_m }],
                  completed_sets, skipped_sets, compliance_pct }],
    totals: { planned_sets, completed_sets, skipped_sets, unregistered_sets,
              planned_duration_ms, actual_duration_ms,
              planned_distance_m, actual_distance_m },
    compliance_pct }
→ 400 INVALID_QUERY · 401 · 403 Forbidden · 404 Not Found · 500

GET /api/v1/activities/comparison?date_from&date_to&team_id&group_id&kind&athlete_user_id
→ 200 ComparisonListResponse
  { date_from, date_to,
    sessions: [{ session_instance_id, date, session_name, team_id, team_name,
                 group_id, group_name, is_presencial, kinds[], status,
                 planned_sets, completed_sets, skipped_sets, unregistered_sets,
                 planned_duration_ms, actual_duration_ms,
                 planned_distance_m, actual_distance_m, compliance_pct }],
    summary: { sessions_planned, sessions_with_activity, avg_compliance_pct } }
→ 400 INVALID_QUERY · 401 · 403 Forbidden · 500
```

`planned_duration_ms` y `planned_distance_m` son `null` si ningún ejercicio tiene
ese tipo de objetivo. Las fechas van en `YYYY-MM-DD` y los horarios en RFC 3339
UTC. Las sesiones de la lista se ordenan por fecha ascendente.

## Risks / Trade-offs

- **Consultas en lote:** la lista hace una consulta por tabla para todas las
  sesiones del rango (sin N+1). Con el tope de 366 días alcanza.
- **El cumplimiento depende del dato cargado:** una serie con el cronómetro
  olvidado (por ejemplo 700 minutos) da 100% por el tope, y la desviación lo
  muestra.
- **Membresía histórica:** depende de que `date_start` y `deleted_at` reflejen
  bien cuándo entró y salió el atleta.

## Migration Plan

No hay cambios de schema. Solo se agregan rutas.

## Open Questions

- Si a futuro se quiere comparar por plan de entrenamiento, primero hay que
  conservar `source_plan_id` al editar un día.
