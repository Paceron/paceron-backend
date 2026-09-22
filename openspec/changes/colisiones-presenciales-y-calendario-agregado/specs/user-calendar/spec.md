# Delta Spec: user-calendar (nueva capability)

## ADDED Requirements

### Requirement: Banners del home — next-session del corredor

El sistema DEBE cumplir lo siguiente (MUST):

`GET /api/v1/users/{id}/next-session` (con `{id}` == usuario autenticado, `403` si no) responde **siempre `200`** con `{next_cancelled, next_training}`, ambos independientes y nullable:

- Cada uno es el más próximo cronológicamente de su `kind` (`cancelled`/`training`) entre TODOS los grupos donde el usuario es miembro activo, con `date >= hoy`.
- Criterio "hoy cuenta": si `date` es hoy y el día es presencial, solo cuenta si `presencial_time_from` todavía no pasó (mismo criterio de día cerrado).
- `next_training` incluye `is_presencial` y, si es presencial, `presencial_time_from`/`presencial_time_to`/`presencial_location`; un training asincrónico los omite/null.
- `next_cancelled` es `{group_id, group_name, date, session_name}` sin campos presenciales.
- `session_name` sale de la instancia congelada; `null` si faltara.

#### Scenario: ambos próximos presentes

- **WHEN** el usuario tiene una próxima sesión cancelada y otra training (de cualquier grupo donde es miembro)
- **THEN** `200` con ambos objetos poblados e independientes.

#### Scenario: sin compromisos

- **WHEN** no hay ni `cancelled` ni `training` próximos
- **THEN** `200` con ambos en `null` (nunca `204`).

### Requirement: Banner del home — next-presencial-session del entrenador

El sistema DEBE cumplir lo siguiente (MUST):

`GET /api/v1/users/{id}/next-presencial-session` (nuevo, `{id}` == autenticado): la próxima `GroupCalendarDay` con `kind='training'`, `is_presencial=true`, `date >= hoy` (mismo criterio de "hoy cuenta") entre TODOS los grupos que administra el usuario (`owner_id` de sus equipos).

- `200` `{group_id, group_name, team_id, team_name, date, session_name, presencial_time_from, presencial_time_to, presencial_location}` — la primera cronológicamente, sin importar equipo.
- `204` si no hay ninguna.

#### Scenario: próxima presencial entre varios equipos

- **WHEN** el entrenador administra grupos en 2+ equipos y solo uno tiene presencial próximo
- **THEN** `200` con ese día, `team_id`/`team_name` del equipo del grupo.

### Requirement: Calendario agregado del corredor

El sistema DEBE cumplir lo siguiente (MUST):

`GET /api/v1/users/{id}/member-calendar?from={date}&to={date}` (nuevo, `{id}` == autenticado, `from`/`to` obligatorios): array de días de TODOS los grupos donde es miembro activo en el rango, cada item con los campos de `GroupCalendarDay` + `group_id`/`group_name`/`team_id`/`team_name`, ordenado por fecha.

#### Scenario: varios grupos en el mismo rango

- **WHEN** el usuario es miembro de 2 grupos con días en el rango
- **THEN** un solo response incluye los días de ambos con los nombres de grupo y equipo resueltos server-side.

### Requirement: Calendario agregado del entrenador con marcado de colisiones

El sistema DEBE cumplir lo siguiente (MUST):

`GET /api/v1/users/{id}/administered-calendar?from={date}&to={date}` (nuevo, `{id}` == autenticado, `from`/`to` obligatorios): mismo shape que member-calendar pero de TODOS los grupos que administra.

- Cada día con `is_presencial=true` que superponga en horario con otro día presencial de otro grupo administrado en la misma fecha trae `presencial_collision: {type: "same_team"|"cross_team", conflicts: [{group_id, group_name, team_id, team_name, presencial_time_from, presencial_time_to}]}`.
- `cross_team` gana si hay de ambos tipos; `conflicts` lista todos los colisionantes.
- Incluye colisiones same-team (permitidas por el guard) y viejas guardadas antes del guard — la detección es sobre datos actuales.
- `presencial_collision` ausente/`null` si no colisiona.

#### Scenario: colisión vieja detectada en la vista

- **WHEN** existen dos días presenciales superpuestos guardados antes de que existiera el guard
- **THEN** `administered-calendar` marca ambos días con `presencial_collision`.
