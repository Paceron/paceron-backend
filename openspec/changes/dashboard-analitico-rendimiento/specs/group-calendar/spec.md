# Spec Delta

## MODIFIED Requirements

### Requirement: Reasignar o borrar un día ya cerrado está prohibido

El sistema SHALL rechazar (`422`) cualquier intento de asignar, reasignar o borrar contenido de un `GroupCalendarDay` cuya fecha ya está "cerrada": `date` pasada, `date` de hoy con `is_presencial=true` y `now >= presencial_time_from`, o `date` de hoy con `is_presencial=false` (async). `is_presencial` participa **solo** en decidir si el día está cerrado — una vez que un día es cerrado, la única operación permitida sobre él es la transición a `kind=cancelled` (sin excepción, independientemente de si era presencial o no); ninguna otra operación de escritura tiene trato especial por `is_presencial`.

Este guard aplica a los **5** endpoints de escritura del calendario: `PUT` de un día individual, `DeleteDay`, `stamp`, `bulk` y `bulk-clear`, y a `shift` sobre cualquier fila que el corrimiento afecte. La transición a `kind=cancelled` sobre un día que ya estaba en `training` SHALL seguir permitida sobre un día cerrado (no reinstancia nada, solo marca `cancelled_reason` conservando el `session_instance_id` existente).

`stamp`/`bulk`/`bulk-clear`/`shift` SHALL validar todas las fechas/filas afectadas contra esta regla antes de escribir cualquiera — si alguna ya está cerrada, el sistema rechaza la operación completa (`422`, con la lista de fechas en conflicto), sin aplicar ningún cambio parcial.

**Excepción acotada — `SEEDING_ENABLED`.** Solo con `--stage=local` y la variable de entorno `SEEDING_ENABLED` en `true`, el guard de "fecha pasada" SHALL quedar relajado y el stamping de fechas históricas SHALL proceder con normalidad. La excepción aplica **únicamente** a la condición `date < hoy`; **no** relaja el resto de las condiciones de cierre (presencial de hoy ya empezada, async de hoy), ni la validación de los otros cuatro endpoints.

La bandera SHALL cumplir tres condiciones, porque desactiva una invariante de negocio y no una limitación técnica:

1. SHALL ser una **variable de entorno del servidor**, nunca un parámetro de la petición. Un parámetro de request podría activarlo cualquier entrenador autenticado.
2. **Si `SEEDING_ENABLED` no está seteada o vale `false`, nada de esto aplica**: la bandera es inerte y el proceso arranca con normalidad, sin ninguna comprobación extra. Esta es la condición normal de todo deploy, y es la que hay que decir explícitamente para que la regla 3 no se lea como un arranque condicional.
3. **Si `SEEDING_ENABLED=true` y el stage NO es `--stage=local`**, el proceso SHALL **negarse a arrancar** (`panic`). Esa es la única combinación que mata el proceso: la bandera está activa en un entorno donde no debe estar.
4. Cada uso SHALL quedar registrado en el log, con el identificador del usuario y las fechas afectadas.

Las tres condiciones en código:

```
SEEDING_ENABLED != true          →  inerte, arranca normal
SEEDING_ENABLED == true && local →  excepción activa
SEEDING_ENABLED == true && !local →  panic
```

#### Scenario: Reasignar un día pasado

- **WHEN** el entrenador dueño intenta `PUT /groups/{id}/calendar/{date}` sobre una fecha pasada que ya tenía `kind=training`
- **THEN** el sistema responde `422`, sin crear ninguna instancia ni modificar la fila existente

#### Scenario: Reasignar un día futuro sí está permitido

- **WHEN** el entrenador dueño reasigna un día futuro que ya tenía una `SessionInstance` (ej. cambia a otra `Session` del catálogo)
- **THEN** el sistema crea una nueva `SessionInstance`/`ExerciseInstance` para ese día y repunta el `session_instance_id`

#### Scenario: Cancelar un día presencial ya empezado

- **WHEN** el entrenador dueño hace `PUT` con `kind=cancelled` sobre un día de hoy, presencial, ya empezado (`now >= presencial_time_from`)
- **THEN** el sistema acepta la transición (no es una reasignación de contenido, solo un cambio de estado)

#### Scenario: Bulk con una fecha cerrada entre varias abiertas
- **WHEN** el entrenador dueño manda `bulk` con 5 fechas, una de ellas ya pasada
- **THEN** el sistema responde `422` con la fecha pasada señalada, sin aplicar el cambio a ninguna de las 5

#### Scenario: Bulk-clear con una fecha cerrada
- **WHEN** el entrenador dueño manda `bulk-clear` con fechas que incluyen un día ya cerrado
- **THEN** el sistema responde `422` con esa fecha señalada, sin borrar ninguna de las fechas del lote

#### Scenario: Shift que movería una fila ya cerrada
- **WHEN** el entrenador dueño corre `shift` desde una fecha tal que al menos una fila con `date >= from_date` ya está cerrada
- **THEN** el sistema responde `422` con esa fila señalada, sin mover ninguna fecha del lote

#### Scenario: Stamping histórico con la bandera activa en local

- **GIVEN** `SEEDING_ENABLED=true` en un entorno local
- **WHEN** el entrenador hace `stamp` de un plan sobre una fecha pasada sin contenido previo
- **THEN** el sistema estampa la fecha, crea las instancias y registra el uso en el log

#### Scenario: La bandera no relaja las demás condiciones de cierre

- **GIVEN** `SEEDING_ENABLED=true`
- **WHEN** se intenta estampar un día de hoy presencial cuya hora de inicio ya pasó
- **THEN** el sistema responde `422` igual que sin la bandera

#### Scenario: La bandera no se acepta como parámetro de la petición

- **WHEN** una petición incluye `seeding_enabled=true` en el body o en el query string
- **THEN** el campo se ignora y el guard de día cerrado se aplica normalmente

#### Scenario: Bandera inerte

- **WHEN** el proceso arranca sin `SEEDING_ENABLED`, o con ella en `false`
- **THEN** arranca con normalidad, sin comprobación de stage, y el guard de fecha pasada sigue intacto

#### Scenario: Arranque fuera de local con la bandera activa

- **WHEN** el proceso arranca con `SEEDING_ENABLED=true` sin `--stage=local` (incluido staging)
- **THEN** el proceso se niega a arrancar y deja constancia del motivo
