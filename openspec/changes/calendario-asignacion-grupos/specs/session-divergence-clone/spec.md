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

### Requirement: Congelamiento automático de días ya ejecutados o en curso

El sistema SHALL, al editar una sesión (`PUT /sessions/{id}`), identificar además — sin que el entrenador tenga que excluirlos a mano — cualquier `GroupCalendarDay` individual que referencie esa sesión y esté "cerrado" según su fecha/horario, y repuntearlo al mismo clon compartido de la edición (creándolo si todavía no existía por no haber `exclude_group_ids`). Un día se considera cerrado si se cumple cualquiera de:
- Su fecha es anterior a hoy.
- Es de hoy, `is_presencial=true`, y la hora actual ya pasó `presencial_time`.
- Es de hoy y `is_presencial=false` (asíncrono) — se considera cerrado todo el día, sin importar la hora.

Un día futuro, o de hoy sin cerrar (presencial antes de su horario), sigue en vivo salvo que su grupo haya sido excluido explícitamente.

#### Scenario: Editar una sesión con un día ya pasado asignado
- **WHEN** se edita una sesión que tiene un `GroupCalendarDay` con fecha anterior a hoy, sin que su grupo esté en `exclude_group_ids`
- **THEN** el sistema clona la sesión (mismo clon compartido que usaría por exclusión manual) y repuntea ese día puntual al clon, dejando el resto de las referencias abiertas intacto

#### Scenario: Sesión presencial de hoy, antes de su horario
- **WHEN** un `GroupCalendarDay` de hoy tiene `is_presencial=true` y la hora actual es anterior a `presencial_time`
- **THEN** ese día sigue en vivo — la edición se aplica normalmente, sin clonar por ese día

#### Scenario: Sesión presencial de hoy, después de su horario
- **WHEN** un `GroupCalendarDay` de hoy tiene `is_presencial=true` y la hora actual ya pasó `presencial_time`
- **THEN** el sistema clona y repuntea ese día, aunque su grupo no haya sido excluido a mano

#### Scenario: Sesión asíncrona de hoy
- **WHEN** un `GroupCalendarDay` de hoy tiene `is_presencial=false`
- **THEN** el sistema lo trata como cerrado y lo clona/repuntea, sin esperar a que termine el día — los corredores pueden haberla ya realizado en cualquier momento de hoy

#### Scenario: Múltiples días cerrados y grupos excluidos en la misma edición
- **WHEN** una edición combina `exclude_group_ids` no vacío con uno o más días cerrados en grupos no excluidos
- **THEN** el sistema crea un único clon compartido por toda la operación — tanto los grupos excluidos a mano como los días cerrados detectados automáticamente terminan apuntando al mismo clon

#### Scenario: Ningún día cerrado ni grupo excluido
- **WHEN** todas las referencias a la sesión son días futuros (o de hoy sin cerrar) y no se excluyó ningún grupo
- **THEN** el sistema no crea ningún clon — aplica la edición directamente a la sesión original, comportamiento sin cambios respecto al caso simple
