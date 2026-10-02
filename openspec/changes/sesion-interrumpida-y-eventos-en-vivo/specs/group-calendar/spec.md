# Delta spec: group-calendar (Gap 26)

Nota: capability existente (ver cambios previos: `asignacion-por-instanciacion`, `instancia-referencia-catalogo`, `colisiones-presenciales-y-calendario-agregado`, `stamp-exclude-dates`, `session-instance-detail`). El resto de la capability queda vigente.

## ADDED Requirements

### Requirement: Estado de apertura/cierre de la sesión presencial

El sistema DEBE cumplir lo siguiente (MUST):

- `group_calendar_days` registra `presencial_opened_at` y `presencial_closed_at` (nullable timestamps). El estado "abierta" es `opened_at != NULL AND closed_at == NULL`; "cerrada" es `closed_at != NULL` (final, sin reopen). Días no presenciales no llevan estado de apertura.
- El detalle `GET /api/v1/session-instances/{id}` expone `presencial_open` (bool) + `opened_at` + `closed_at` cuando hay día asociado; en día no presencial o instancia huérfana los campos quedan fuera del JSON (omitempty, para no contaminar las respuestas de calendario que comparten el DTO de instancia).

#### Scenario: detalle refleja apertura

- **WHEN** el corredor abre tarde el pre-start y consulta `GET /session-instances/{id}` de un día presencial abierto
- **THEN** la respuesta trae `presencial_open: true`, `opened_at` seteada y `closed_at` ausente (o null si el día ya cerró, con `presencial_open: false`).

#### Scenario: detalle sin día o no presencial

- **WHEN** la instancia no tiene día de calendario asociado (huérfana) o el día no es presencial
- **THEN** los campos de apertura no aparecen en el JSON.

### Requirement: Evento WS de estado de sesión

El sistema DEBE cumplir lo siguiente (MUST):

- Al abrir y al cerrar una sesión presencial (por las acciones del owner descritas en la capability `runnersession`), el backend emite `update:session_state` al canal `session:{id}` con data `{presencial_open, opened_at, closed_at}` (estado post-escritura), sin excluir al emisor.

#### Scenario: sala de espera en vivo

- **WHEN** el owner abre la sesión y hay corredores suscriptos al canal `session:{id}`
- **THEN** reciben `update:session_state` con `presencial_open: true` y pueden unirse sin recargar.

#### Scenario: corredor conectado después del cierre

- **WHEN** un corredor se conecta (o consulta el detalle) después de que la sesión cerró
- **THEN** el estado consultado es `presencial_open: false` con `closed_at` seteada — no depende de haber visto el mensaje en vivo.
