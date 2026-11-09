# tier-management

## ADDED Requirements

### Requirement: Tiers del rol corredor ocultos y protegidos

El sistema DEBE (MUST) conservar un tier base interno para el rol `corredor` como soporte de asignación y permisos, pero NO DEBE exponerlo ni aceptar tiers del corredor por la API de catálogo.

#### Scenario: Catálogo filtrado por corredor devuelve vacío

- **WHEN** un cliente autenticado llama `GET /api/v1/tiers?role_id=<id de corredor>`
- **THEN** el sistema responde `200` con `[]`
- **AND** no devuelve el tier base interno ni tiers pagos del corredor

#### Scenario: Catálogo sin filtro oculta tiers del corredor

- **WHEN** un cliente autenticado llama `GET /api/v1/tiers` sin `role_id`
- **THEN** la respuesta no contiene ningún tier cuyo `role_name` sea `corredor`
- **AND** conserva los tiers de otros roles

#### Scenario: No se crean tiers del corredor

- **WHEN** un cliente envía `POST /api/v1/tiers` con el `role_id` de `corredor`
- **THEN** el sistema responde un error de validación
- **AND** no inserta el tier

#### Scenario: No se modifican ni eliminan tiers del corredor por API

- **WHEN** un cliente intenta `PUT /api/v1/tiers/{id}` o `DELETE /api/v1/tiers/{id}` sobre un tier de `corredor`
- **THEN** el sistema responde un error de validación
- **AND** el tier base interno permanece intacto

### Requirement: Asignación y cambio de tier del corredor

El sistema DEBE (MUST) rechazar tiers explícitos para el rol `corredor` y DEBE seguir asignando el tier base interno cuando no se envía `tier_id`.

#### Scenario: Asignación sin tier usa el base

- **WHEN** un usuario asigna el rol `corredor` sin `tier_id`
- **THEN** la asignación se crea con el tier base interno
- **AND** `GET /auth/permissions` resuelve el tier base y sus permisos sin error

#### Scenario: Asignación con tier explícito se rechaza

- **WHEN** un usuario asigna el rol `corredor` con un `tier_id`
- **THEN** el sistema responde un error de validación
- **AND** no crea `user_roles` ni suscripciones

#### Scenario: Cambio de tier del corredor se rechaza

- **WHEN** un usuario intenta `PUT /api/v1/users/{id}/roles/{role_id}/tier` sobre el rol `corredor`
- **THEN** el sistema responde un error de validación
- **AND** no modifica suscripciones ni cuotas

#### Scenario: Entrenador conserva tiers

- **WHEN** un usuario asigna o cambia tiers del rol `entrenador`
- **THEN** el comportamiento existente se mantiene sin cambios
