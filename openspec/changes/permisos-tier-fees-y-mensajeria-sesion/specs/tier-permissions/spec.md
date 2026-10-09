# tier-permissions delta

## ADDED Requirements

### Requirement: Listado de permisos de un tier

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /api/v1/tiers/:id/permissions` devuelve los permisos asignados al tier indicado, activos (`deleted_at IS NULL`), con `permission_id` y `permission_name`.
- Acceso: cualquier usuario autenticado (sin ownership — el listado es descriptivo y el frontend lo usa sobre tiers ajenos).
- Si el tier no existe → `404`. Si el tier existe sin permisos → `200` con lista vacía.

#### Scenario: Listado feliz

- **WHEN** un usuario autenticado consulta `GET /tiers/3/permissions` de un tier con permisos asignados
- **THEN** recibe `200` con `{"permissions":[{"permission_id":N,"permission_name":"..."}]}` ordenado por `permission_id` ASC, solo permisos activos.

#### Scenario: Tier inexistente

- **WHEN** el id de tier no existe
- **THEN** responde `404`.

#### Scenario: Tier sin permisos

- **WHEN** el tier existe y no tiene permisos activos
- **THEN** responde `200` con `{"permissions":[]}`.
