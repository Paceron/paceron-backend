## Context

Change pequeño y acotado sobre el mecanismo de `asignacion-por-instanciacion` (ya implementado y en `develop`). Dos pedidos del frontend (Gap 7, `paceron-frontend/docs/BACKEND_API_GAPS.md`), confirmados en código: referencia de origen de la instancia, y conservación de la instancia actual al guardar sin `session_id` (`PUT` individual y `bulk`).

## Decisions

### D1. Referencia de origen como columna nullable gestionada por la app

`session_instances.source_session_id` y `exercise_instances.source_exercise_id`: `*int64`, nullable, sin constraint de FK — mismo patrón exacto que `GroupCalendarDay.source_plan_id` y que las FK opacas de `workout_feedback`. El pedido del frontend decía "`ON DELETE SET NULL`, mismo patrón que `source_plan_id`": en este repo ese patrón **es** app-managed sin DDL de constraint (los catálogos `Session`/`Exercise` solo se soft-borran, `deleted_at`; la fila nunca desaparece físicamente, así que un `SET NULL` de DB no dispararía jamás). No se limpia la referencia al soft-borrar el catálogo: queda informativa, y el frontend determina si "sigue existiendo" consultando el catálogo (que ya filtra `deleted_at IS NULL`).

Se pueblan en `instantiateSession` (único punto de creación de instancias): `source_session_id` = id de la `Session` de catálogo resuelta; `source_exercise_id` = id de cada `Exercise` copiado. Sin backfill para instancias existentes (D6 de `asignacion-por-instanciacion` aplicaría igual: no hay dato reconstructible confiable — dos instancias del mismo día pueden haber salido de sesiones distintas o de una edición intermedia).

### D2. Exposición en la respuesta D9: `session_id` / `exercise_id`

`SessionInstanceResponse` suma `session_id *int64` (json `"session_id"`), `InstanceExerciseResponse` suma `exercise_id *int64` — nombres pedidos por el frontend, no `source_*` (el prefijo `source_` es detalle de persistencia; en la respuesta `session_id` dentro del objeto `session_instance` no es ambiguo porque el `id` de la instancia ya tiene su propio campo). Cambios puramente aditivos al contrato: clientes viejos los ignoran.

### D3. Conservación de instancia en `UpsertDay` (`PUT` individual)

Regla nueva: `kind=training` con `session_id == nil` es válido **si y solo si** el día ya tiene `session_instance_id` no-nulo; en ese caso se conserva tal cual la instancia (no se instancia, no se borra nada, no se toca `workout_feedback`). Con `session_id` presente (igual o distinto al origen), comportamiento idéntico al actual: reinstanciar de cero + borrar la vieja si no tiene feedback.

Mecánica en `calendar_service.go`:
- `validateDayFields` pasa a recibir `hasExistingInstance bool`; la rama `training` devuelve `ErrCalendarFieldMismatch` solo si `req.SessionID == nil && !hasExistingInstance`.
- En la transacción de `UpsertDay`: flag local `preservada bool`; rama `training` con `req.SessionID == nil` → `row.SessionInstanceID = txExisting.SessionInstanceID`, `preservada = true`. La condición de borrado de instancia superada (`txExisting.SessionInstanceID != nil && kind != cancelled`) se extiende con `&& !preservada` — sin esto, conservar borraría la instancia que se quiere conservar (cuando la fila queda con el mismo `session_instance_id`, el `Upsert` no la cambia pero el `deleteSupersededInstance` sí la atacaría).
- El pre-check fuera de transacción (llamada a `validateDayFields` con `existing`) usa el mismo flag, y el path mock (`s.db == nil`) espeja la conservación igual que ya espeja la de `cancelled`.

### D3bis. `bulk` con la misma conservación, por fecha

`POST .../bulk` con `kind=training` y `session_id == nil`: cada fecha del lote conserva su propia instancia. `Bulk` ya valida por fecha en su loop (con `existing[i]`), así que pasa `hasExistingInstance = existing[i] != nil && existing[i].SessionInstanceID != nil` por fecha. Si alguna fecha NO tiene instancia que conservar, se rechaza el lote completo (all-or-nothing, como las fechas cerradas): nuevo error `ErrCalendarTrainingWithoutInstance` (wrapea el patrón de `newCalendarClosedDaysError`: lista las fechas en el mensaje, mapea a `422`). Con `session_id` presente el comportamiento de `bulk` no cambia (una instancia nueva por fecha). Stamp no toca esta regla: su `session_id` viene siempre del `PlanDay`.

### D4. Guards y permisos intactos

Ningún cambio en `isCalendarDayClosed`, cancelación sobre cerrado, atomicidad de lotes, ni en el borrado con feedback (D3/D10 de `asignacion-por-instanciacion` siguen palabra por palabra). El `PUT` conservado sigue siendo una escritura sobre día abierto (un día cerrado con `kind=training` sigue bloqueado por el guard, con o sin `session_id`).

## Risks / Trade-offs

- Un `PUT` que conserva la instancia y a la vez cambia `is_presencial`/horarios es ahora posible sin reinstanciar — correcto por diseño (la instancia no depende de lo presencial).
- `session_id` en la respuesta puede apuntar a una `Session` soft-borrada: documentado en D1, el contrato lo declara "referencia informativa".
- El frontend no puede distinguir "conservar" vs "reinstanciar con la misma sesión" si manda `session_id` == origen siempre: es su elección de payload; el backend expone ambos caminos.

## Migration Plan

Solo `AutoMigrate` (suma 2 columnas nullable, sin datos que migrar). Rollback: dejar de escribir/leer las columnas; las filas sobreviven con `source_*` poblados sin efecto. Sin deploy aislado de partes: modelos + service + respuesta van juntos en el mismo deploy (las columnas nullable permiten que una versión vieja del código ignore las nuevas).

## Open Questions

Ninguna — los dos pedidos del frontend tienen diseño cerrado en D1-D3; los nombres de JSON los fijó el pedido explícito de Gap 7.
