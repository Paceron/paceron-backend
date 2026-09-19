## D1: Columnas viejas se dropean, sin backfill

`presencial_time`/`default_time` (único) no tienen forma de convertirse limpiamente a un par `from`/`to` — no hay dato para inferir el horario de fin. Se dropean directamente (`ALTER TABLE ... DROP COLUMN IF EXISTS`, raw SQL post-AutoMigrate en `postgres.go`, mismo patrón ya usado para otras migraciones de este archivo). Cualquier fila presencial cargada en testing con el esquema viejo pierde su horario y debe recargarse a mano — aceptable dado que no hay actividad real todavía (ver proposal.md).

## D2: Validación `time_to > time_from`

Nueva regla, no existía con el campo único. Sentinels nuevos por dominio: `ErrPlanInvalidTimeRange` (training plan), `ErrCalendarInvalidTimeRange` (calendar) — ambos `422`. Se valida en el mismo punto donde ya se validaba el formato `HH:MM` (`validateAndBuildDays` en training plan, `validateDayFields` en calendar), no en un paso separado.

## D3: Bug preexistente encontrado — lectura de `*time.Time` sin `.UTC()` corrompe la hora

**Síntoma:** `TestSessionService_Update_TodayPresencialAfterStartTimeLocks` (ya existente, D13) falla o pasa de forma no determinística dependiendo de la hora del día en que corre la suite — reproducido contra Postgres real, en `develop` HEAD, sin ningún cambio de este change.

**Causa raíz:** estos campos se escriben con `time.Parse("15:04", "10:15")`, que devuelve un `time.Time` con `Location() == UTC` y fecha `0000-01-01`. Postgres almacena esto como `timestamptz` (instante absoluto). Al leerlo, el driver (`lib/pq`, vía GORM) devuelve el valor convertido a `time.Local` del proceso — en este repo, `America/Argentina/Cordoba` (UTC-3). Reproducido de forma aislada:

```
WROTE hour=10 min=15 loc=UTC
READ  hour=7 min=15 loc=Local raw=2000-01-01 07:15:00 -0300 -03
```

El mismo instante, pero `.Hour()`/`.Minute()` devuelven un valor distinto al escrito porque `.Location()` cambió. Cualquier código que llame `.Hour()`/`.Minute()`/`.Format(...)` directo sobre el valor leído sin normalizar primero obtiene una hora incorrecta — en año artificial `0000`/`0001` (como usa esta app para "horario suelto sin fecha"), el offset ni siquiera es un `-3h` limpio: cae en el offset LMT (Local Mean Time, pre-estandarización) de la zona para esa fecha tan antigua (`-04:16` observado), agravando la corrupción.

**Por qué no se detectó antes:** los tests que aseveran el contenido de un `PresencialTime`/`DefaultTime` usan mocks (sin Postgres real, sin este problema). Los únicos tests con Postgres real que tocan estos campos (los 3 de auto-lock de D13) solo assertan un booleano (¿se clonó o no?) — nunca el valor de hora en sí. La corrupción solo volteaba ese booleano en ciertas franjas horarias, haciendo el test intermitente en vez de romperse siempre.

**Alternativas evaluadas para el fix:**
1. **Columna Postgres `time`/`time without time zone` nativa** (probada primero): elimina la ambigüedad de zona de raíz, pero `lib/pq` no soporta escanear ese tipo directo a `*time.Time` (`unsupported Scan, storing driver.Value type string into type *time.Time`) — requeriría un tipo Go custom con `sql.Scanner`/`driver.Valuer`, más invasivo. Descartada.
2. **Normalizar a un año moderno (ej. 2000) en vez de año 0/1** (evaluada): resuelve el caso LMT específico, pero el bug base (offset UTC→Local en cualquier año) sigue intacto — 10:15 UTC seguiría leyéndose como 07:15. Insuficiente por sí sola.
3. **`.UTC()` en cada lectura antes de extraer hora/minuto** (elegida): dado que SIEMPRE se escribe en UTC (convención ya implícita en el código existente vía `time.Parse`), forzar la misma convención al leer hace que el round-trip sea determinístico y correcto para cualquier año, sin tocar el tipo de columna ni requerir un scanner custom. Mínimo cambio, máxima corrección.

**Aplicado en:** `toCalendarDayResponse`, `toPlanDayResponse` (servicios), `isCalendarDayClosed` (D13, `session_service.go`), `NextSession`. Documentado como convención obligatoria en los comentarios de `dbs.GroupCalendarDay`/`dbs.PlanDay` — cualquier lectura nueva de estos campos debe seguir el mismo patrón.

## D4: Alcance del fix del bug D3

Se corrige únicamente en el código tocado por este change (los 4 puntos de lectura de `PresencialTime*`/`DefaultTime*`). No se auditaron otros campos `*time.Time` del resto del backend por estar fuera de alcance — si aparece un síntoma similar en otro dominio, tratar como bug aparte.
