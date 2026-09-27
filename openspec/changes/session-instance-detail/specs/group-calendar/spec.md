## ADDED Requirements

> Nota: se declara como `ADDED` (no `MODIFIED`) porque la capability `group-calendar` nunca fue archivada en `openspec/specs/` — ver convención en `AGENTS.md` §3.

### Requirement: Detalle standalone de una sesión instancia por id

El sistema DEBE cumplir lo siguiente (MUST).

`GET /session-instances/{id}` con un id de instancia existente SHALL responder `200` con el shape completo de la instancia: `{id, session_id (nullable, origen catálogo), name, description (nullable), created_at, exercises: [...]}`, donde cada ejercicio incluye `{id, exercise_id (nullable, origen catálogo), name, kind, description, intensity, minutes, distance_m, speed_kph, muscle_group, video_url, role, repeat_count, rest_minutes}` — idéntico al shape que el calendario embebe como `session_instance`. Un id inexistente SHALL responder `404`. Un id existente sin acceso SHALL responder `403`.

El acceso SHALL otorgarse por la regla dual: (a) existe un día de calendario con esa instancia y el caller es miembro activo del grupo del día, u owner del equipo del grupo; o (b) existe un feedback activo (no soft-deleted) sobre la instancia y el caller es atleta del feedback, reportante del feedback, u owner del equipo del feedback. Ningún otro caso SHALL otorgar acceso (no se expone contenido entre equipos sin uno de esos vínculos).

#### Scenario: Atleta consulta la instancia de un ítem de su historial

- **GIVEN** un atleta con un feedback sobre una instancia huérfana (el día fue reasignado; la instancia vive sin día)
- **WHEN** hace `GET /session-instances/{id}` con el `session_instance_id` del historial
- **THEN** responde `200` con el shape completo (nombre, descripción, `session_id` y ejercicios con role/repeat_count/rest_minutes)

#### Scenario: Instancia de un día del grupo del atleta

- **GIVEN** un atleta miembro activo de un grupo cuyo día `2026-10-07` tiene la instancia asignada
- **WHEN** hace `GET /session-instances/{id}`
- **THEN** responde `200` con el shape completo, sin depender de feedback

#### Scenario: Entrenador consulta la instancia de un día de su equipo

- **GIVEN** el owner de un equipo con un grupo que tiene la instancia asignada en un día
- **WHEN** hace `GET /session-instances/{id}`
- **THEN** responde `200` con el shape completo

#### Scenario: Instancia inexistente

- **WHEN** se pide `GET /session-instances/999999999`
- **THEN** responde `404` sin revelar nada más

#### Scenario: Sin acceso

- **GIVEN** una instancia asignada a un día de un grupo de otro owner, sin feedback
- **WHEN** un usuario autenticado sin ningún vínculo la pide
- **THEN** responde `403`

#### Scenario: Membresía vencida no da acceso

- **GIVEN** un ex-miembro de un grupo (membresía con `date_end` pasado) cuyo grupo tiene la instancia
- **WHEN** hace `GET /session-instances/{id}`
- **THEN** responde `403` (el 403 por membresía vencida no depende del feedback)

#### Scenario: Feedback soft-deleted no da acceso

- **GIVEN** un feedback sobre la instancia que fue soft-deleted, sin día que la referencie ni otro vínculo
- **WHEN** el ex-atleta del feedback la pide
- **THEN** responde `403`
