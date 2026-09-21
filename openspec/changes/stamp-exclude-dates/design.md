## Design

### D1 — `exclude_dates` es un filtro sobre las fechas objetivo, antes de todo lo demás

`Stamp` calcula `targetDates[i] = start_date + (SequenceNo_i - 1)` sobre `planDays`. Con `exclude_dates`, se parsea el set (formato `YYYY-MM-DD`; formato inválido → `ErrCalendarInvalidDate` → `422`), y **luego se filtra la lista de pares (índice de planDay, fecha)** a las que no están en el set. Todo el flujo posterior (pre-validación de `planReq`, construcción de `rows`, guard de cerradas, guard de conflictos/`force`, loop de escritura e instanciación, borrado de instancias superadas, respuesta) opera sobre el subconjunto filtrado, sin cambios de lógica. Fechas del set que no caen en el rango → simplemente nunca filtran nada (ignorar silenciosamente).

Consecuencias directas de filtrar tan temprano:
- Día excluido que además estaba cerrado o ocupado → no dispara `422` ni `409` (no se evalúa), y su fila/instancia quedan intactas porque el loop de escritura y `deleteSupersededInstance` solo itera el subconjunto.
- Subconjunto vacío (todas las fechas excluidas, o plan de 1 día excluido) → `201` con `[]` (no se considera error; acordado explícitamente).

### D2 — Validación del array

- `nil` o `[]` → set vacío → comportamiento idéntico al actual (requisito de compatibilidad).
- Cada elemento DEBE parsear como `YYYY-MM-DD` exacto (mismo layout que `start_date`); uno inválido aborta antes de tocar nada con un sentinel nuevo, `ErrCalendarInvalidDate` (`"exclude_dates debe tener formato YYYY-MM-DD"`) → `422`.
- Duplicados: inofensivos (set).

### D3 — Path mock (`s.db == nil`)

Los tests de service con mock DAO cubren el camino de conflictos del mock (`:699-727`); el filtro se aplica igualmente ahí (sobre `targetDates` antes de calcular min/max y conflictos) para que ambos caminos compartan semántica. El resto del mock (que devuelve error de "no hay DB") no cambia.

### D4 — Contrato Swagger

`@Param body` del stamp documenta `exclude_dates` opcional con ejemplo; `@Failure 422` agrega "fecha inválida en exclude_dates". Regenerar `cmd/api/docs`.
