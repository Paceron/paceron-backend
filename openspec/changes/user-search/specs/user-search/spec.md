## ADDED Requirements

### Requirement: Búsqueda de usuarios activos por texto parcial
El sistema SHALL exponer `GET /api/v1/users/search?q=<texto>` para cualquier usuario autenticado, sin restricción de rol adicional, devolviendo coincidencias parciales case-insensitive por nombre, apellido o email entre usuarios con `status = active`.

#### Scenario: Búsqueda válida con resultados
- **WHEN** un usuario autenticado busca con `q` de al menos 3 caracteres que coincide parcialmente con el nombre, apellido o email de uno o más usuarios activos
- **THEN** el sistema retorna HTTP 200 con hasta 5 resultados, cada uno con `user_id`, `name`, `surname`, `email`

#### Scenario: Búsqueda sin resultados
- **WHEN** ningún usuario activo coincide con el texto buscado
- **THEN** el sistema retorna HTTP 200 con una lista vacía

#### Scenario: Texto de búsqueda demasiado corto
- **WHEN** `q` tiene menos de 3 caracteres (después de recortar espacios) o está vacío
- **THEN** el sistema retorna HTTP 400 sin ejecutar la búsqueda

#### Scenario: Usuario no autenticado
- **WHEN** se llama al endpoint sin `Authorization: Bearer <access_token>` válido
- **THEN** el sistema retorna HTTP 401, igual que cualquier otra ruta protegida

#### Scenario: Usuarios inactivos excluidos
- **WHEN** el texto buscado coincide con un usuario cuyo `status` no es `active` (inactive/pause/blocked/suspended)
- **THEN** ese usuario no aparece en los resultados
