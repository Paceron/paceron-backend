# Delta spec: runnersession (Gaps 19 y 26)

Nota: capability nueva; el comportamiento previo (`wip`/`finished` con POST/PATCH/GET idempotentes) queda vigente salvo lo modificado acá.

## ADDED Requirements

### Requirement: Estado interrumpido del corredor

El sistema DEBE cumplir lo siguiente (MUST):

- PATCH `/api/v1/session-instances/{id}/runner` admite `status` con valores `"finished"` e `"interrupted"`.
- Transiciones válidas: `wip→finished`, `wip→interrupted`, `interrupted→finished`. Toda transición desde `finished` es rechazada con 400. `interrupted→interrupted` y `finished→finished` son idempotentes (200 sin cambios).
- `end_date` la setea el servidor al pasar a `interrupted` o a `finished` (incluida `interrupted→finished`, con el momento de esa transición explícita).
- La respuesta mantiene el shape `RunnerSessionResponse` plano actual; el mensaje de la mutación de interrupción es `"sesión marcada como interrumpida"`.
- La matriz de autorización no cambia: self por default; `athlete_user_id` ajeno solo si el auth es owner de un equipo al que pertenece el atleta.
- Una fila `interrupted` NO es rejugable: el POST idempotente devuelve la fila existente sin reabrir.

#### Scenario: corredor interrumpe su sesión

- **WHEN** un corredor con sesión `wip` hace PATCH con `{"status":"interrupted"}`
- **THEN** responde 200 con `status:"interrupted"`, `end_date` seteada por el servidor, y su feedback de series ya registradas permanece intacto.

#### Scenario: transición explícita de interrupted a finished

- **WHEN** un corredor con sesión `interrupted` hace PATCH con `{"status":"finished"}`
- **THEN** responde 200 con `status:"finished"` y `end_date` actualizada al momento de esa transición.

#### Scenario: no se interrumpe una sesión finalizada

- **WHEN** un corredor con sesión `finished` hace PATCH con `{"status":"interrupted"}`
- **THEN** responde 400 sin modificar la fila.

### Requirement: Sesión presencial controlada por el entrenador (apertura y cierre)

El sistema DEBE cumplir lo siguiente (MUST):

- El día de calendario presencial (`kind=training`, `is_presencial=true`) registra `presencial_opened_at` y `presencial_closed_at` (nullable).
- El **owner del team** del grupo del día, al crear su propio estado de sesión (su Play), setea `presencial_opened_at` (solo si es NULL) y queda exento del gate de apertura.
- Al pasar el owner a `finished`, se setea `presencial_closed_at` (solo si es NULL). El `interrupted` del owner NO cierra la sesión. No existe reopen: una vez cerrada, permanece cerrada.
- Un corredor NO-owner no puede crear su estado de sesión en un día presencial si la sesión no fue abierta (409, code `session_not_opened`) o ya fue cerrada (409, code `session_closed`). Días no presenciales o de otro `kind` no llevan gate.
- Los slugs viajan en el campo `code` del error; el texto es adicional.

#### Scenario: owner abre la sesión presencial

- **WHEN** el owner del equipo del grupo hace POST `/runner` sobre el día presencial
- **THEN** su estado se crea en `wip` y el día queda con `presencial_opened_at` seteada (idempotente: re-Play no la reescribe).

#### Scenario: corredor intenta unirse antes de la apertura

- **WHEN** un corredor no-owner hace POST `/runner` sobre un día presencial con `opened_at` NULL
- **THEN** responde 409 con `code:"session_not_opened"` y no se crea su fila.

#### Scenario: corredor intenta unirse después del cierre

- **WHEN** un corredor no-owner hace POST `/runner` sobre un día presencial con `closed_at` seteada
- **THEN** responde 409 con `code:"session_closed"` y no se crea su fila.

#### Scenario: el owner interrumpe sin cerrar

- **WHEN** el owner hace PATCH `{"status":"interrupted"}` sobre su sesión de un día presencial
- **THEN** su fila pasa a `interrupted` y el día NO queda cerrado (`presencial_closed_at` permanece NULL).
