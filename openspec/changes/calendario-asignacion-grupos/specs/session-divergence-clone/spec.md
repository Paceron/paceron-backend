## ADDED Requirements

### Requirement: Descubrir grupos afectados antes de editar una sesión asignada

El sistema SHALL exponer `GET /sessions/{id}/assigned-groups` devolviendo, distinct, todos los grupos que tienen al menos una `GroupCalendarDay.session_id` igual a esa sesión, sin importar la fecha.

#### Scenario: Sesión sin asignaciones
- **WHEN** una sesión nunca fue estampada ni asignada a ningún calendario de grupo
- **THEN** `GET /sessions/{id}/assigned-groups` devuelve un array vacío

#### Scenario: Sesión asignada en 2 grupos
- **WHEN** una sesión aparece en el calendario de 2 grupos distintos (en cualquier cantidad de fechas)
- **THEN** el sistema devuelve exactamente esos 2 grupos, sin repetir

### Requirement: Editar una sesión con exclusión de grupos crea un clon compartido

El sistema SHALL, cuando `PUT /sessions/{id}` recibe `exclude_group_ids` no vacío, crear un único clon de la sesión (con `clone_name`/`clone_description` si vienen, o el sufijo default), repuntear `GroupCalendarDay.session_id` de esos grupos al clon, y recién después aplicar el resto de la edición a la sesión original — todo en una sola transacción.

#### Scenario: Editar sin excluir grupos
- **WHEN** `PUT /sessions/{id}` no trae `exclude_group_ids` (u omitido/vacío)
- **THEN** el sistema aplica la edición directamente a la sesión original, sin crear ningún clon — todos los grupos que la tenían asignada ven el cambio

#### Scenario: Editar excluyendo 3 grupos
- **WHEN** `PUT /sessions/{id}` trae `exclude_group_ids` con 3 grupos
- **THEN** el sistema crea un único clon, repuntea las `GroupCalendarDay` de esos 3 grupos al mismo clon, y aplica la edición a la sesión original — los grupos no listados y el catálogo en general quedan con el cambio en la sesión original

#### Scenario: Falla el paso final de la edición
- **WHEN** el `PUT` de los campos normales de la sesión falla después de haber creado el clon y repunteado los grupos
- **THEN** el sistema revierte toda la operación — ni el clon ni el repunteo quedan aplicados
