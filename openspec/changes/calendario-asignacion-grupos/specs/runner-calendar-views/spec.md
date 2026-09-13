## ADDED Requirements

### Requirement: Próximo entrenamiento del corredor entre todos sus grupos

El sistema SHALL exponer `GET /users/{id}/next-session` devolviendo la `GroupCalendarDay` con `kind IN (training, cancelled)` de fecha más próxima (`>= hoy`) entre todos los grupos de los que `{id}` es miembro, o `204` si no hay ninguna.

#### Scenario: Con próxima sesión
- **WHEN** el usuario autenticado tiene al menos un grupo con una `GroupCalendarDay` futura de tipo `training`
- **THEN** el sistema devuelve `200` con esa fila, la más próxima entre todos sus grupos

#### Scenario: Sin ninguna sesión futura
- **WHEN** ningún grupo del usuario tiene una fila `training`/`cancelled` con fecha `>= hoy`
- **THEN** el sistema responde `204`

#### Scenario: Consultar el next-session de otro usuario
- **WHEN** el `{id}` de la URL no coincide con el usuario autenticado del token
- **THEN** el sistema responde `403`

### Requirement: Resumen de calendarios para "Mis asignaciones"

El sistema SHALL exponer `GET /users/{id}/calendar-summary` devolviendo un ítem `{group_id, group_name}` por cada grupo del que `{id}` es miembro, sin calendario embebido.

#### Scenario: Corredor con varios grupos
- **WHEN** el usuario autenticado es miembro de 3 grupos
- **THEN** el sistema devuelve un array de 3 ítems, uno por grupo, cada uno resoluble después con `GET /groups/{id}/calendar`
