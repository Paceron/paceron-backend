## ADDED Requirements

### Requirement: Persistir visibilidad de grupos para corredores
El sistema SHALL persistir un flag `show_groups_to_runners` por equipo, configurable en la creación y actualización, con default `false`.

#### Scenario: Crear equipo sin especificar el campo
- **WHEN** se envía POST a `/api/v1/teams` sin `show_groups_to_runners` en el body
- **THEN** el equipo se crea con `show_groups_to_runners: false`

#### Scenario: Crear equipo especificando el campo
- **WHEN** se envía POST a `/api/v1/teams` con `"show_groups_to_runners": true`
- **THEN** el equipo se crea con `show_groups_to_runners: true`

#### Scenario: Actualizar el campo en un equipo existente
- **WHEN** se envía PUT a `/api/v1/teams/:id` con `"show_groups_to_runners": true`
- **THEN** el equipo actualiza el flag y `GET /api/v1/teams/:id` lo refleja en respuestas subsiguientes
