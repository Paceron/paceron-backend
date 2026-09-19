## Why

Actualización de spec del frontend (`docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md`, 2026-09-19): `presencial_time` (único) pasa a `presencial_time_from`/`presencial_time_to` — toda sesión presencial necesita horario de inicio Y fin, no un solo horario. Mismo cambio en `PlanDay.default_time` → `default_time_from`/`default_time_to` (`docs/BACKEND_TRAINING_PLANS_SPEC.md`).

Durante la implementación se encontró y corrigió un bug real preexistente, no relacionado con el split en sí pero descubierto por sus tests: `PresencialTime`/`DefaultTime` se escriben en UTC (`time.Parse("15:04", ...)` sin zona) pero Postgres las devuelve convertidas al `time.Local` del proceso al leerlas — en `America/Argentina/Cordoba` (UTC-3) esto corrompía la hora leída en cada round-trip, dependiendo de la hora del día en que corriera el proceso. Ver design.md D3.

## What Changes

- `GroupCalendarDay.PresencialTime` → `PresencialTimeFrom`/`PresencialTimeTo`; `PlanDay.DefaultTime` → `DefaultTimeFrom`/`DefaultTimeTo`. Columnas viejas (`presencial_time`, `default_time`) se dropean — no hay migración limpia de un único valor a un rango.
- Nueva validación: `is_presencial`/`default_presencial = true` exige ambos horarios, y `time_to` debe ser posterior a `time_from` (`422` si no) — antes solo se exigía un horario.
- **Fix de bug preexistente**: todas las lecturas de estos campos (`toCalendarDayResponse`, `toPlanDayResponse`, `isCalendarDayClosed`, `NextSession`) ahora llaman `.UTC()` antes de extraer hora/minuto — sin esto, el corte de D13 (congelamiento automático por horario) podía fallar silenciosamente según la hora del día en que se ejecutara el proceso.
- 4 DTOs actualizados (`PlanDayRequest`/`Response`, `CalendarDayRequest`/`Response`, `BulkRequest`, `NextSessionResponse`), controllers, swagger, docs.

## Capabilities

### Modified Capabilities

- `group-calendar` (de `calendario-asignacion-grupos`): `presencial_time` → `presencial_time_from`/`presencial_time_to` en `GroupCalendarDay` y en los endpoints que lo tocan (`PUT`/`bulk`/`stamp`/`next-session`).
- `training-plan-catalog` (de `catalogo-planes-entrenamiento`): `default_time` → `default_time_from`/`default_time_to` en `PlanDay`.

## Non-Goals

- No se agrega backfill de datos: no había actividad real (calendarios presenciales cargados) dependiendo del valor viejo en ningún ambiente desplegado — el bug de timezone descrito arriba significa que cualquier hora presencial ya cargada podía estar mostrando un valor corrompido de todos modos.
- No se introduce un tipo `time` nativo de Postgres (se evaluó, `lib/pq` no puede escanear `time`/`time without time zone` directo a `*time.Time` sin un scanner custom) — se mantiene `timestamptz` con la convención de interpretarlo siempre en UTC, documentada en el modelo (`dbs.GroupCalendarDay`/`dbs.PlanDay`).
