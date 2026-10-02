# Delta spec: users (Gap 23)

Nota: capability nueva; el comportamiento previo de `GET /users` (search) y `GET /users?ids=` (batch lookup) queda vigente salvo lo agregado acá.

## ADDED Requirements

### Requirement: Foto en sugerencias y roster

El sistema DEBE cumplir lo siguiente (MUST):

- `SearchResultItem` (compartido por search y batch lookup) incluye `photo_url`, nullable: `null` si el usuario no tiene foto de perfil.
- La URL se arma con la misma regla que el resto de los media del repo (URL pública del bucket con `?v=` derivado de `photo_updated_at`).

#### Scenario: usuario con foto en el roster

- **WHEN** el frontend consulta `GET /users?ids=` de usuarios con foto
- **THEN** cada ítem trae `photo_url` distinto de null, armado con la misma URL pública `?v=` que expone el update response.

#### Scenario: usuario sin foto en el roster

- **WHEN** el frontend consulta usuarios sin foto de perfil
- **THEN** el ítem trae `photo_url: null` y el resto del shape queda igual.
