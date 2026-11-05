## Why

Hoy `isCalendarDayClosed` cierra **siempre** un día async de la fecha actual, igual que un día pasado: el entrenador no puede asignar ni reasignar contenido para hoy aunque él mismo vea el calendario y decida que todavía quiere hacerlo. Para presencial ya existe la regla correcta (se cierra recién al pasar `presencial_time_from`); para async no hay ninguna razón de negocio para el bloqueo duro. Es el Gap 39 de `paceron-frontend/docs/BACKEND_API_GAPS.md`.

## What Changes

- **El día async de hoy deja de estar cerrado**: `isCalendarDayClosed` devuelve `false` para `date == hoy && !is_presencial`.
- **Se habilita cualquier escritura del calendario sobre ese día**, no solo asignar: `PUT`, `DELETE`, `stamp`, `bulk`, `bulk-clear` y `shift`, porque el guard es único y de nivel día. El entrenador decide si opera o espera a mañana.
- **Sin cambios en el resto del guard**: pasado sigue cerrado (salvo `SEEDING_ENABLED`), futuro abierto, presencial de hoy se cierra recién al pasar `presencial_time_from`, y `cancelled` sigue siendo la excepción permitida sobre un día cerrado.
- **Sin cambios en lectura**: los banners/`next-session` mantienen su criterio actual ("hoy async cuenta"). No hay cambio de contrato de respuesta ni de forma de request.

## Capabilities

### Modified Capabilities

- `group-calendar`: el guard de día cerrado deja de bloquear la escritura sobre un día async de hoy.

## Impact

- `cmd/api/services/calendar_service.go`: única rama de producción (`isCalendarDayClosed`) + comentario de la función.
- Tests: unitarios del guard (`calendar_service_seeding_test.go`, `calendar_service_task3_test.go`), rediseño del caso mixto de `calendar_service_cobertura_test.go`, comentario del caso de `exclude_dates`, y regresiones nuevas de escritura real sobre hoy async.
- Docs: `docs/CATALOGO_Y_CALENDARIO.md` §8.3 y `docs/FRONTEND_IMPACTO_INSTANCIACION.md` §4.
- Sin migración, sin cambios de DTOs, sin regeneración de Swagger.

## No alcance

- Cambiar el criterio de lectura/banners de "hoy cuenta".
- Partir el guard por operación: todas las escrituras comparten el mismo día cerrado.
- Abrir días pasados sin `SEEDING_ENABLED`, ni presenciales de hoy ya iniciadas.
- Tocar el comportamiento del frontend: ya envía la request y el bloqueo es 100% backend.

## Métrica de éxito

- Un entrenador puede asignar, reasignar, borrar, estampar, correr o limpiar un día async de hoy sin `422 ErrCalendarDayClosed`.
- Presencial de hoy ya iniciada sigue devolviendo `422`; pasado y futuro conservan su comportamiento.
- Suite completa y gate de coverage verdes; `openspec validate --strict` válido.
