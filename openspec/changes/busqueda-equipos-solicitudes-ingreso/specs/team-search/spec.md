## ADDED Requirements

### Requirement: Búsqueda paginada de equipos visibles

El sistema SHALL permitir a cualquier usuario autenticado buscar equipos (`GET /api/v1/teams/search`) por filtros opcionales de `name` (parcial, case-insensitive), `level`, `country`, `province` y `city`. Solo SHALL devolver equipos con `visible = true` y sin `deleted_at`. El sistema SHALL excluir de los resultados los equipos donde el caller ya es miembro activo.

#### Scenario: Búsqueda sin filtros
- **WHEN** un usuario autenticado llama `GET /api/v1/teams/search` sin parámetros
- **THEN** el sistema devuelve la primera página de equipos con `visible = true`, excluyendo aquellos donde el caller ya es miembro

#### Scenario: Equipo no visible nunca aparece
- **WHEN** un equipo tiene `visible = false`
- **THEN** no aparece en los resultados de búsqueda sin importar los filtros aplicados

#### Scenario: Caller ya es miembro del equipo
- **WHEN** el usuario autenticado ya pertenece a un equipo que matchea los filtros de búsqueda
- **THEN** ese equipo no aparece en los resultados

### Requirement: Paginación por página, sin conteo total

El sistema SHALL paginar los resultados de búsqueda por `page` (1-indexado, tamaño fijo de 20 resultados por página) y SHALL responder con `has_more` en vez de un conteo total de resultados.

#### Scenario: Hay más resultados que la página actual
- **WHEN** existen más de 20 equipos que matchean los filtros
- **THEN** la respuesta de la página 1 incluye `has_more: true` y como máximo 20 equipos

#### Scenario: Última página
- **WHEN** los resultados restantes caben en la página solicitada
- **THEN** la respuesta incluye `has_more: false`

#### Scenario: Parámetro de página inválido
- **WHEN** se solicita `page` menor a 1
- **THEN** el sistema responde 400 con el código `INVALID_QUERY`

### Requirement: Cada resultado incluye datos para la card de búsqueda

El sistema SHALL incluir en cada resultado el nombre del equipo, ícono (si tiene), nivel, ubicación, cupo actual (miembros actuales sobre `max_members`), y el nombre del entrenador dueño (`owner_name`).

#### Scenario: Resultado con entrenador dueño
- **WHEN** se devuelve un equipo en los resultados de búsqueda
- **THEN** el resultado incluye `owner_name` resuelto desde `users` por `owner_id`, y el conteo actual de miembros
