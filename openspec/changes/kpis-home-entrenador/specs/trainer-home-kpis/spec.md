# trainer-home-kpis Specification

## Requirements

### Requirement: El backend SHALL entregar un resumen del Home del entrenador

El backend SHALL exponer `GET /api/v1/dashboard/trainer-home-kpis` para la
identidad autenticada, sin aceptar IDs de owner/equipo/grupo en el request.
SHALL aceptar `period=month|week|day` y `reference_date=YYYY-MM-DD`, con los
mismos defaults y cortes calendario del Home del corredor: mes calendario,
semana lunes-domingo y día de referencia.

La respuesta SHALL incluir período/rango/días considerados; sesiones
programadas y realizadas (totales y presenciales); equipos y corredores activos;
y cobrado neto y cuotas pendientes. SHALL devolver ceros en vez de omitir claves
cuando no haya datos.

#### Scenario: Obtener el resumen mensual

- **WHEN** un entrenador autenticado solicita el resumen sin parámetros
- **THEN** recibe el resumen del mes actual y todos los campos agregados

#### Scenario: Seleccionar semana o día

- **WHEN** solicita `period=week` o `period=day` con una fecha de referencia
- **THEN** recibe los agregados del lunes-domingo o del día correspondiente

#### Scenario: Parámetros inválidos

- **WHEN** el período o la fecha no son válidos
- **THEN** recibe `400` con un código estable y no un resumen parcial

### Requirement: Los contadores de sesiones SHALL distinguir planificación y realización

Una sesión programada SHALL ser un día de calendario `training` con instancia,
no cancelado, dentro del período y del scope de equipos activos del entrenador.
Una sesión realizada SHALL ser una sesión programada con al menos una asistencia
registrada. Las presenciales SHALL aplicar `is_presencial=true` además de esas
reglas y ser un subconjunto de las sesiones totales.

#### Scenario: Sesión realizada

- **WHEN** una sesión programada tiene una o más asistencias registradas
- **THEN** suma una sesión realizada (una vez, no una vez por corredor)

#### Scenario: Cancelada, descanso o sin instancia

- **WHEN** un día es cancelado, descanso o no tiene instancia de sesión
- **THEN** no suma como programado ni realizado

### Requirement: Los totales actuales SHALL estar aislados por entrenador

`active_teams` SHALL contar equipos del entrenador no eliminados y activos.
`active_runners` SHALL contar corredores activos y únicos entre esos equipos.
Un corredor compartido entre dos equipos SHALL contarse una sola vez.

#### Scenario: Otro entrenador y equipos inactivos

- **WHEN** existen equipos de otros owners o equipos del caller eliminados/inactivos
- **THEN** no afectan los contadores del caller

### Requirement: Las cobranzas SHALL mostrar neto aprobado y deuda del período

`net_collected` SHALL sumar pagos de membresías de equipo aprobados y con
`payment_id` válido, creados en el período, restando la comisión de Paceron.
`pending_amount/count` SHALL representar obligaciones no pagadas cuyo
vencimiento cae dentro del período. Los límites de cobros SHALL interpretarse en
la zona horaria de Córdoba/Argentina.

#### Scenario: Cálculo de neto

- **WHEN** el período incluye pagos aprobados y otros estados o preferencias sin `payment_id`
- **THEN** solo los aprobados válidos suman a neto y la comisión se resta una sola vez

#### Scenario: Obligaciones pendientes

- **WHEN** una cuota no pagada vence dentro del período
- **THEN** aumenta el monto y la cantidad pendientes; cuotas pagadas o con vencimiento fuera del período no suman
