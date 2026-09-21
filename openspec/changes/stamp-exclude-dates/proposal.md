## Why

`POST /groups/{id}/calendar/stamp` es todo-o-nada sobre el rango completo del plan: con `force=true` pisa **todos** los días que ya tengan contenido, y sin `force` cualquier día ocupado rechaza el stamp entero con `409`. No hay forma de decirle "estampá el plan, pero dejá estos días puntuales tal como están". El frontend ya construye el preview de estampado y pidió el campo para "evitar pisar selectivo" (Gap 8 en `paceron-frontend/docs/BACKEND_API_GAPS.md`) — hoy la única alternativa sería un rodeo de dos escrituras (`force=true` + restaurar después vía `PUT`), descartado por frágil.

## What Changes

- **`StampRequest` suma `exclude_dates`** (`[]string`, opcional, formato `YYYY-MM-DD`): cada fecha del set que caiga dentro del rango objetivo del plan se salta por completo — no se crea, no se modifica, no se borra su instancia; si el día ya tenía una fila queda exactamente como estaba.
- **Las fechas excluidas no participan de ninguna guarda**: no cuentan para el `409` de conflictos, ni para el guard de día cerrado (`422`), ni se validan contra `validateDayFields`. El filtro se aplica antes de construir los `rows`.
- **La respuesta no incluye las fechas excluidas** (no se tocaron, no hay nada nuevo que devolver).
- **Comportamiento invariante si el campo se omite o viene vacío**: set vacío = cero filtrado, idéntico a hoy (contrato aditivo, compatible).
- Decisiones de casos borde (acordadas): fecha con formato inválido en el array → `422` (`ErrCalendarInvalidDate`, nuevo sentinel — no se toca el manejo actual de `start_date`); fecha excluida fuera del rango del plan → se ignora silenciosamente; rango resultante vacío (todas las fechas excluidas) → `201` con array vacío, no es error.

## Capabilities

### Modified Capabilities

- `group-calendar` (de `asignacion-por-instanciacion` / `instancia-referencia-catalogo`): el estampado de planes permite excluir fechas puntuales del rango.

## Impact

- DTO: `cmd/api/domains/calendar/calendar_day_request.go` (`StampRequest` +1 campo opcional).
- Service: `cmd/api/services/calendar_service.go` (`Stamp` — filtro previo a guards/escritura, nuevo sentinel `ErrCalendarInvalidDate`; path mock `db == nil` refleja el filtro).
- Controller: `mapCalendarError` mapea el nuevo sentinel a `422`; anotaciones Swagger del stamp + regenerar `cmd/api/docs`.
- Docs: `docs/CATALOGO_Y_CALENDARIO.md` §8 (semántica de stamp), `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (nota aditiva, cerrar Gap 8 del lado frontend).
- Tests: service real-Postgres (exclusión simple, fecha excluida cerrada/ocupada no dispara guards, fecha inválida 422, fuera de rango ignorada, rango vacío → array vacío, regresión: sin el campo nada cambia).
- Frontend (`paceron-frontend`): desbloquea "evitar pisar selectivo" en `stamp-plan-modal.jsx` — sin acción obligatoria para clientes viejos (omitir el campo = comportamiento exacto de hoy).

## Non-Goals

- Excluir por `sequence_no` del plan (la exclusión es por fecha calendario, como la pidió el frontend).
- Cambiar la semántica de `force` sobre las fechas NO excluidas (sigue igual que hoy).
- Agregar `exclude_dates` a `bulk` o al `PUT` individual (no tiene sentido: ahí el cliente ya elige qué fechas escribir).
- Arreglar el manejo de `start_date` con formato inválido (hoy cae a 500 con error genérico) — deuda preexistente, fuera de alcance.
