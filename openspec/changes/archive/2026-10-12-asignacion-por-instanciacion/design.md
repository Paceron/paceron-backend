## D1: Tablas de instancia — shape y por qué separadas del catálogo

```
exercise_instances          session_instances            session_exercise_instances
├─ id PK                    ├─ id PK                     ├─ id PK
├─ name                     ├─ name                      ├─ session_instance_id FK
├─ description              ├─ description               ├─ exercise_instance_id FK
├─ kind                     └─ created_at                ├─ role
├─ intensity                                              ├─ repeat_count
├─ minutes                                                └─ rest_minutes
├─ distance_m
├─ speed_kph
├─ muscle_group
├─ video_url
└─ created_at
```

Sin `owner_id` (no son catálogo de nadie — nacen de una copia, no de una creación de usuario). Sin `deleted_at` ni `updated_at`: una instancia nunca se edita después de creada, y su borrado (D3) es físico, no lógico — no hace falta filtrar por "activo" en ningún lado porque no hay UI que las liste directamente (solo se llega a ellas navegando desde `GroupCalendarDay`).

**Separadas de `exercises`/`sessions`/`session_exercises`, no una columna `is_instance`** en las mismas tablas: así `GET /exercises?owner_id=`/`GET /sessions?owner_id=` nunca necesitan un filtro adicional, y una instancia no puede aparecer por error en un listado de catálogo aunque alguien olvide un `WHERE`. Mismo razonamiento que ya se usó para separar `GroupCalendarDay` del catálogo en el change original.

## D2: `isCalendarDayClosed` cambia de trigger-de-clonado a guard-de-escritura

Se reusa la función tal cual (misma regla de fecha/horario ya validada: pasado siempre cerrado, presencial de hoy cerrado desde su `presencial_time_from`, async de hoy siempre cerrado, futuro nunca cerrado — ver `calendario-asignacion-grupos` D13), pero cambia dónde vive y qué hace con el resultado:

- **Antes** (`session_service.go`, D13): al editar la `Session`, para cada día ya cerrado que la referenciaba → clonar y repuntear ese día, dejando pasar la edición para los demás.
- **Ahora** (se mueve a `calendar_service.go`): al intentar escribir `kind=training` (o cualquier `IsPresencial=true`) sobre un día de calendario — `PUT`/`Bulk`/`Stamp` —, si ese día **ya existe y está cerrado**, rechazar con `422 ErrCalendarDayClosed` antes de tocar nada. No importa si el `session_id` pedido es el mismo que ya tenía o uno distinto: reasignar (con el mismo u otro contenido) un día cerrado no está permitido, punto — coherente con "no se puede cargar retroactivamente qué pasó" (non-goal ya existente).
- `cancelled` sigue permitido sobre un día cerrado que estaba en `training` — cancelar no reinstancia nada, solo marca `cancelled_reason` y conserva el `session_instance_id` existente como contexto (igual que hoy).
- `DeleteDay` sobre un día cerrado también se bloquea con el mismo guard, por simetría (no se puede "borrar la historia" tampoco).
- `Stamp`/`Bulk` validan **todas** las fechas del lote contra este guard antes de escribir cualquiera — si alguna fecha objetivo ya existe y está cerrada, se rechaza el lote entero con la lista de fechas en conflicto (mismo patrón UX que el `409` de conflicto de `Stamp` por contenido existente, no un silent-skip).

## D3: Borrado de instancia superada — safety check contra `workout_feedback`

Al reasignar un día futuro que ya tenía `session_instance_id` (nunca puede ser un día cerrado, por D2), la instancia vieja se borra físicamente **si y solo si** ningún `workout_feedback.assigned_session_id`/`assigned_exercise_id` la referencia todavía. Confirmado con el usuario: dado que la reasignación solo aplica a futuro, este caso debería ser improbable en la práctica (normalmente hay feedback cuando el día ya pasó) — pero se implementa el chequeo igual, es barato y evita reabrir exactamente el tipo de bug que motivó todo este rework.

Esto es **preparación**, no wiring real: `workout_feedback` sigue con sus FK opacas tal cual están hoy (`assigned_session_id > 0`, sin join real) — este change no lo toca. El chequeo de "¿tiene feedback?" se implementa contra la tabla `workout_feedback` igual (columnas `assigned_session_id`/`assigned_exercise_id` ya existen), aun sin que haya una FK real todavía; es una lectura defensiva, no requiere que el modelo esté wireado.

Si la instancia vieja SÍ tiene feedback: no se borra, queda huérfana (ningún `GroupCalendarDay` activo la referencia, pero sigue resolviendo para quien la busque por ID desde el feedback histórico). No es un estado de error, es el resultado esperado — mismo criterio que ya se usa para soft-delete de `Exercise`/`Session` en el catálogo (dejar de listarse, seguir resolviendo).

## D4: Una instancia por día, sin deduplicación

Si un `save` del frontend asigna la misma `Session` a 10 días distintos, se crean 10 `SessionInstance` independientes (y sus `SessionExerciseInstance`/`ExerciseInstance`, también sin compartir). Alternativa descartada: deduplicar dentro del mismo `save` cuando el contenido es idéntico — se descarta porque reintroduce acoplamiento entre días (editar/borrar la instancia de un día podría afectar a otro que la comparte), exactamente lo que este rework busca evitar. Costo aceptado: más filas — decisión explícita del usuario, "no es lo más cómodo pero son bases sólidas".

## D5: Qué se borra del código existente

No queda conviviendo con el mecanismo nuevo — se elimina:

- `SessionRequest.ExcludeGroupIDs`/`CloneName`/`CloneDescription` y toda la rama de `SessionService.Update` que los procesa (D8).
- `isCalendarDayClosed` tal como vive hoy en `session_service.go`, y su uso como trigger tanto en `SessionService.Update` (D13) como en `ExerciseService.Update` (`congelar-ejercicio-en-clon`) — se recicla la lógica de fecha/horario (D2) pero no el mecanismo de clonado.
- `cloneSessionInternal`'s deep-clone de `Exercise` (`deepCloneExercises`) — sin objeto de clonado histórico que resolver, `POST /sessions/{id}/clone`/`POST /exercises/{id}/clone` (duplicar-para-editar, sin relación con esto) quedan como estaban antes de `congelar-ejercicio-en-clon`.
- `GroupCalendarDaoInterface.FindBySessionID`/`FindByExerciseID`/`RepointSessionForGroups`/`RepointDaysByID` — sin usuarios una vez removido lo anterior.
- Tests correspondientes en `session_service_test.go`/`exercise_service_test.go`/`group_calendar_day_dao_test.go`.
- Secciones §5/§8 de `docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md` (frontend) y §8 de `docs/CATALOGO_Y_CALENDARIO.md` (este repo) que documentan el mecanismo de clonado — se reemplazan por la documentación del mecanismo de instanciación, no se dejan las dos conviviendo.

## D7: `GET /sessions/{id}/assigned-groups` se elimina

Existía únicamente para el paso 1 del flujo D8 (checklist de "qué grupos excluir" antes de editar una `Session` con asignaciones activas). Ese flujo entero desaparece con este change — no queda ninguna razón para pedir "qué grupos tienen esta sesión asignada" antes de editarla, porque editar la sesión ya no afecta ninguna asignación existente. Se elimina el endpoint, `CalendarServiceInterface.AssignedGroups`, `GroupCalendarDaoInterface.FindDistinctGroupsBySession` y el handler correspondiente en `session_controller.go` — sin reemplazo, no cumple ninguna función bajo el modelo nuevo.

## D8: El guard de día cerrado aplica a los 5 endpoints de escritura, no solo a 3

`BulkClear` y `Shift` quedaron sin mencionar explícitamente en la primera versión de esta spec — ambigüedad real, no una omisión intencional. Se extiende el mismo guard (D2) a los 5:

- `PutDay`, `Bulk`, `Stamp`: no pueden crear ni reemplazar contenido `training`/`other`/`rest` sobre un día cerrado (ya cubierto).
- `BulkClear`: es la versión en lote de `DeleteDay` — mismo criterio exacto, todo-o-nada sobre el lote.
- `Shift`: mueve fechas siempre hacia adelante (`days` positivo, ya validado), nunca hacia atrás — pero si alguna fila **afectada por el corrimiento** (`date >= from_date`) ya está cerrada, moverla equivale a reescribir retroactivamente cuándo pasó algo que ya pasó. Se bloquea con el mismo criterio todo-o-nada: si al menos una fila afectada está cerrada, se rechaza el `Shift` completo (`422`, listando esas fechas), no se mueve ninguna.

La única operación exenta del guard sobre un día cerrado sigue siendo la transición a `cancelled` (D2) — no crea ni reemplaza instancia, solo cambia `kind`/`cancelled_reason` conservando el `session_instance_id` existente.

**Aclaración de redacción** (ambigüedad detectada en la implementación): la condición que define "¿este día está cerrado?" usa `is_presencial`/`presencial_time_from` (D2, regla de fecha/horario) — eso es completamente independiente de qué operaciones están permitidas *una vez que* un día ya es cerrado. Una vez cerrado, la única operación permitida es `cancelled`, sin excepción, sin importar si ese día era presencial o no — `is_presencial` solo participa en decidir si el día está cerrado, no en qué se puede hacer después.

## D9: Shape de `GetRange`/`NextSession` — embeben el detalle completo de la instancia

Decisión pendiente en la versión original de esta spec (marcada como "a definir en implementación" en `tasks.md` Task 3) — se cierra acá porque bloqueaba escribir las DTOs: **`CalendarDayResponse`/`NextSessionResponse` embeben el detalle completo de la `SessionInstance` y sus `ExerciseInstance`, no solo un ID.**

Es la única opción viable: no hay ningún endpoint público de catálogo que resuelva una instancia (son tablas internas, sin exponer), y pegarle a `GET /sessions/{id}` para "resolver" el `session_instance_id` sería exactamente el bug que este change existe para cerrar — ese endpoint devuelve el catálogo **en vivo**, no lo que efectivamente se congeló para ese día.

Shape:

```json
{
  "...": "...campos existentes de CalendarDayResponse...",
  "session_instance": {
    "id": 123,
    "name": "Fartlek 5K",
    "description": null,
    "exercises": [
      {"id": 456, "name": "Trote", "kind": "jogging", "description": null, "intensity": null, "minutes": 10, "distance_m": null, "speed_kph": null, "muscle_group": null, "video_url": null, "role": "warmup", "repeat_count": 1, "rest_minutes": 0}
    ]
  }
}
```

`session_instance` es `null` cuando `kind != training` (y cuando `kind = cancelled` sin sesión previa, caso que no debería darse dado que `cancelled` solo se alcanza desde `training`). Mismo shape para `NextSessionResponse`. **Rompe el contrato actual del frontend** (`session_id` bare) — coordinar con `paceron-frontend` antes de mergear, no es un cambio aditivo.

## D10: Orden exacto de borrado de instancia superada

D3 ya establece *que* se borra la instancia vieja salvo que tenga feedback — acá se fija el orden exacto dentro de la transacción, para que nunca haya un momento donde algo apunte a una fila ya borrada:

1. Crear la instancia nueva completa (`SessionInstance` + `SessionExerciseInstance` + `ExerciseInstance`, las 3 tablas).
2. Actualizar `GroupCalendarDay.session_instance_id` para que apunte a la instancia nueva.
3. Recién ahí chequear si algún `workout_feedback` referencia la instancia **vieja** (`assigned_session_id` = ID de la `SessionInstance` vieja, o `assigned_exercise_id` IN los IDs de sus `ExerciseInstance`).
4. Si no hay feedback: borrar en orden hijo-antes-que-padre — `SessionExerciseInstance` de la instancia vieja, después sus `ExerciseInstance` (todas le pertenecen en exclusiva, D4 garantiza que una instancia nunca comparte `ExerciseInstance` con otra), y por último la fila de `SessionInstance` vieja.
5. Si hay feedback: no se borra nada de la instancia vieja — queda huérfana (D3).

## D11: Migración de datos

Mismo criterio que `presencial-time-from-to`: sin backfill. `GroupCalendarDay.session_id` apuntaba al catálogo — no hay forma de "instanciar retroactivamente" contenido que nunca se congeló, y no hay datos reales de producción dependiendo de esto (`testing` stage, feature de calendario recién shipeada). Se dropea la columna vieja por SQL crudo post-`AutoMigrate`, igual patrón que los otros drops recientes en `postgres.go`.
