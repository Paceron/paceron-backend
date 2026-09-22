# Delta Spec: group-calendar (modificada)

## ADDED Requirements

### Requirement: Validación de colisión presencial entre grupos del mismo entrenador

El sistema DEBE cumplir lo siguiente (MUST):

Al escribir (`PUT /groups/{id}/calendar/{date}`, `POST /groups/{id}/calendar/stamp`, `POST /groups/{id}/calendar/bulk`, `POST /groups/{id}/calendar/shift`) un día que queda con `is_presencial=true`, el backend busca entre TODOS los grupos administrados por el mismo `owner_id` otro día con `kind='training'`, `is_presencial=true`, misma fecha y rango horario superpuesto.

- El overlap es `(from < other.to) && (other.from < to)` — bordes que se tocan (termina 09:00, arranca 09:00) NO son colisión.
- Los días `kind='cancelled'` quedan fuera de la detección en ambos lados (ni como escritura a validar ni como colisionante).
- Un día no presencial (o que deja de serlo) nunca dispara la detección.
- El grupo escrito se excluye a sí mismo; en `shift` se excluyen las filas movidas (por ID) y se evalúan las fechas nuevas.

#### Scenario: colisión contra grupo de otro equipo rechaza con 409

- **WHEN** la escritura deja un día presencial superpuesto con un día presencial de un grupo de OTRO equipo del mismo owner
- **THEN** la escritura se rechaza con `409` y body `{"message": "colisión presencial con otro equipo", "conflicts": [{group_id, group_name, team_id, team_name, date, presencial_time_from, presencial_time_to}]}`, sin forma de forzar, sin escritura parcial.

#### Scenario: superposición con grupo del mismo equipo guarda con warning

- **WHEN** la única superposición es contra grupos del MISMO equipo que el grupo escrito
- **THEN** la escritura se guarda y la respuesta exitosa incluye `same_team_warnings` con los conflictos en la misma forma que `conflicts` (campo ausente si no hay).

#### Scenario: all-or-nothing en operaciones por lote

- **WHEN** en `bulk`/`shift` CUALQUIER fecha del lote colisiona con un grupo de otro equipo
- **THEN** se rechaza el lote completo con `409` listando todas las fechas conflictivas (no solo la primera), sin escritura parcial.

#### Scenario: stamp valida colisión después del 409 de conflictos

- **WHEN** un stamp tiene tanto días ocupados sin `force` como colisión potencial
- **THEN** se responde primero el `409` de conflictos existente; la colisión presencial se evalúa solo sobre los días que efectivamente van a quedar presenciales.

### Requirement: same_team_warnings en respuestas de escritura

El sistema DEBE cumplir lo siguiente (MUST):

Las respuestas exitosas de escrituras que dejan días presenciales DEBEN poder incluir los avisos no bloqueantes de superposición same-team:

- `PUT` individual responde `CalendarDayResponse` con campo extra opcional `same_team_warnings` (`omitempty`).
- `stamp`/`bulk`/`shift` responden wrapper `{days: [...], same_team_warnings: [...]}` (coordinar con frontend; ver proposal).

#### Scenario: warning presente solo cuando hay superposición same-team

- **WHEN** la escritura se guarda con superposición contra grupo del mismo equipo
- **THEN** la respuesta incluye `same_team_warnings` con cada colisionante (grupo, equipo, fecha, rango horario); sin superposición same-team, el campo no aparece.
