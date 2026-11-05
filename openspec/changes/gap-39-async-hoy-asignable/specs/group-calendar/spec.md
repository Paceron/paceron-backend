## ADDED Requirements

> Nota: se declaran como `ADDED` (no `MODIFIED`) porque la capability `group-calendar` nunca fue archivada en `openspec/specs/` — misma convención que `asignacion-por-instanciacion` y `stamp-exclude-dates`. El requirement reemplaza la regla vigente de día cerrado para el caso async de hoy.

### Requirement: El día async de hoy no está cerrado para escritura

`isCalendarDayClosed` SHALL devolver `false` cuando la fecha del día sea la fecha actual y el día no sea presencial. En consecuencia, todas las operaciones de escritura del calendario SHALL permitirse sobre un día async de hoy: upsert individual, borrado individual, stamp, bulk, bulk-clear y shift. La decisión de operar o esperar a mañana queda a discreción del entrenador.

El resto de la regla de día cerrado SHALL mantenerse intacto: una fecha anterior a hoy sigue cerrada salvo `SEEDING_ENABLED`; una fecha futura sigue abierta; un día presencial de hoy se cierra recién cuando ya pasó su `presencial_time_from`; un día presencial de hoy sin horario sigue abierto; y la transición a `kind=cancelled` sigue permitida sobre cualquier día cerrado.

#### Scenario: Asignar contenido a un día async de hoy

- **GIVEN** un grupo sin contenido asignado para la fecha actual
- **WHEN** el entrenador dueño hace `PUT /groups/{id}/calendar/{hoy}` con `kind=training` y una sesión de catálogo
- **THEN** el sistema responde `200`, persiste el día async y crea su instancia, sin `422 ErrCalendarDayClosed`

#### Scenario: Reasignar, borrar, estampar, mover o limpiar un día async de hoy

- **GIVEN** un grupo con un día async de hoy ya asignado
- **WHEN** el entrenador dueño lo reasigna, lo borra, lo incluye en un `stamp`, lo corre con `shift`, o lo limpia con `bulk-clear`
- **THEN** la operación se permite y no devuelve `422 ErrCalendarDayClosed` por esa fecha

#### Scenario: Presencial de hoy ya iniciada sigue cerrada

- **GIVEN** un día presencial de hoy cuyo `presencial_time_from` ya pasó
- **WHEN** el entrenador intenta escribir sobre ese día
- **THEN** el sistema responde `422 ErrCalendarDayClosed` con la fecha en el mensaje

#### Scenario: Pasado y futuro conservan su comportamiento

- **GIVEN** un día de ayer y un día de mañana
- **WHEN** se intenta escribir sobre cada uno sin `SEEDING_ENABLED`
- **THEN** ayer devuelve `422 ErrCalendarDayClosed` y mañana se permite

#### Scenario: Las operaciones batch siguen siendo todo-o-nada

- **GIVEN** un lote de fechas que incluye un día async de hoy y un día pasado
- **WHEN** se ejecuta `bulk`, `stamp`, `bulk-clear` o `shift`
- **THEN** el día de hoy async se considera abierto, el día pasado sigue cerrado, y si el lote incluye ese pasado se rechaza completo con la lista de fechas cerradas sin escribir nada

#### Scenario: Los banners de lectura no cambian

- **WHEN** se consultan `next-session` o `next-presencial-session` sobre un grupo con un día async de hoy
- **THEN** el día async de hoy sigue contando según el criterio de lectura vigente, independientemente de que ahora sea escribible
