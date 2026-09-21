## ADDED Requirements

> Nota: se declaran como `ADDED` (no `MODIFIED`) porque la capability `group-calendar` nunca fue archivada en `openspec/specs/` — ver convención en `AGENTS.md` §3. Estos requirements **extendían** el estampado de planes de `asignacion-por-instanciacion`: la semántica de `plan_id`/`start_date`/`force` se mantiene intacta cuando `exclude_dates` se omite o viene vacío.

### Requirement: El estampado de planes permite excluir fechas puntuales del rango

`POST /groups/{id}/calendar/stamp` acepta un campo opcional `exclude_dates` (array de fechas `YYYY-MM-DD`). Cada fecha del set que coincida con una fecha objetivo del rango del plan SHALL saltarse por completo: el sistema no crea, no modifica ni borra nada de ese día (su fila de calendario y su instancia, si existían, quedan intactas). Las fechas excluidas SHALL quedar fuera de todas las guardas del stamp: no cuentan para el `409` de conflictos con `force=false`, ni para el `422` de días cerrados, ni se validan contra las reglas de campos del día. La respuesta SHALL contener solo los días efectivamente escritos (las fechas excluidas no aparecen). Omitir el campo o enviarlo vacío SHALL producir el comportamiento exactamente idéntico al previo al campo.

Una entrada de `exclude_dates` con formato distinto de `YYYY-MM-DD` SHALL rechazarse con `422` (`ErrCalendarInvalidDate`) antes de escribir nada. Una fecha excluida válida que no cae dentro del rango objetivo del plan SHALL ignorarse silenciosamente. Un rango resultante vacío (todas las fechas objetivo excluidas) SHALL responder `201` con array vacío — no es un error. `force` mantiene su semántica actual sobre las fechas NO excluidas.

#### Scenario: Excluir fechas puntuales estampando con force

- **GIVEN** un grupo con días ya asignados en `2026-10-07` y `2026-10-09` dentro del rango que cubriría el plan desde `2026-10-05`
- **WHEN** el entrenador dueño hace `POST .../stamp` con `{plan_id, start_date: "2026-10-05", force: true, exclude_dates: ["2026-10-07", "2026-10-09"]}`
- **THEN** el resto del rango se estampa normalmente, los días `2026-10-07` y `2026-10-09` conservan su fila e instancia intactas (mismo `session_instance_id`), y la respuesta no los incluye

#### Scenario: Fecha excluida cerrada u ocupada no dispara guards

- **GIVEN** un día del rango que está cerrado (fecha pasada o presencial de hoy ya iniciada) y otro con contenido preexistente
- **WHEN** se estampa el plan con `force=false` incluyendo ambas fechas en `exclude_dates`
- **THEN** el stamp procede sin `422` por el día cerrado ni `409` por el día ocupado (las fechas excluidas no se evalúan), y ambos días quedan intactos

#### Scenario: exclude_dates con fecha de formato inválido

- **WHEN** se envía `stamp` con `exclude_dates: ["2026/10/07"]`
- **THEN** el sistema responde `422` (`ErrCalendarInvalidDate`) sin escribir nada

#### Scenario: Fecha excluida fuera del rango del plan se ignora

- **GIVEN** un plan de 7 días desde `2026-10-05`
- **WHEN** se estampa con `exclude_dates: ["2026-12-25"]`
- **THEN** el stamp se comporta exactamente como si el array estuviera vacío

#### Scenario: Rango totalmente excluido responde array vacío

- **GIVEN** un plan cuyos días objetivo son todos incluidas en `exclude_dates`
- **WHEN** se estampa
- **THEN** el sistema responde `201` con `[]` y no escribe nada

#### Scenario: Sin exclude_dates el comportamiento es idéntico al actual (regresión)

- **WHEN** se estampa con `force=true` y sin `exclude_dates` (o con `[]`) sobre un rango con días ocupados y/o cerrados
- **THEN** aplican exactamente los mismos guards (`422` si hay días cerrados, pisado total si `force`) y la respuesta incluye todos los días del plan, como antes del campo
