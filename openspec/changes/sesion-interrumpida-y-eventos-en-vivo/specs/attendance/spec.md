# Delta spec: attendance (Gaps 25 y 28)

Nota: capability nueva; el comportamiento previo (QR, búsqueda, grilla de sesión, bulk manual, borrado) queda vigente salvo lo modificado acá.

## ADDED Requirements

### Requirement: Ventana de membresía por fecha de sesión

El sistema DEBE cumplir lo siguiente (MUST):

- La ventana de membresía de grupo (`group_users.date_start`/`date_end`, timestamptz) se evalúa contra la fecha de la sesión **por día calendario**, no por timestamp: un corredor cuya membresía inició el mismo día de la sesión (con cualquier hora) es miembro activo en esa fecha.
- El criterio aplica a TODO uso del helper de membresía: roster de la grilla (`GET /attendance/session/{id}`), check de registro individual y rechazo del bulk.

#### Scenario: corredor del mismo día aparece en la grilla

- **WHEN** un grupo tiene corredores con membresía iniciada el mismo día de la sesión (con hora posterior a medianoche) y el entrenador consulta `GET /attendance/session/{id}`
- **THEN** todos los corredores activos en esa fecha aparecen en el roster, con su estado `attended` o `not_confirmed` según corresponda.

#### Scenario: corredor del mismo día puede registrar asistencia

- **WHEN** un corredor cuya membresía inició hoy (con hora) registra asistencia por QR, o el entrenador lo incluye en una carga manual
- **THEN** el registro se acepta (la membresía en la fecha de sesión es válida) y no se rechaza por ventana de membresía.

### Requirement: Evento WS de asistencia

El sistema DEBE cumplir lo siguiente (MUST):

- Al registrar asistencia por QR, en cada alta de carga manual y al borrar una asistencia, el backend emite `update:attendance_event` al canal `session:{session_instance_id}`.
- El payload del evento es EXACTO la fila del roster de la grilla: `{user_id, status, source, registered_at, attendance_id}`.
  - Registro: `{user_id, status:"attended", source:"qr"|"manual", registered_at, attendance_id}`.
  - Borrado: `{user_id, status:"not_confirmed", source:null, registered_at:null, attendance_id:null}` (el status infiere la baja).
- La carga manual emite UN evento por corredor afectado (creado o actualizado).
- El registro por QR idempotente (200 sin alta) NO emite evento: no hay cambio de estado.
- El evento no excluye al emisor y es fire-and-forget (mismo patrón que `update:set_event`).

#### Scenario: QR genera evento de alta

- **WHEN** un corredor registra asistencia por QR con éxito
- **THEN** los suscriptos al canal `session:{id}` reciben `update:attendance_event` con la fila `{user_id, status:"attended", source:"qr", registered_at, attendance_id}`.

#### Scenario: carga manual genera un evento por corredor

- **WHEN** el entrenador guarda asistencia manual para N corredores (altas y actualizadas)
- **THEN** se emiten N eventos, cada uno con la fila de ese corredor (`source:"manual"`).

#### Scenario: borrado genera evento de baja

- **WHEN** el entrenador borra una asistencia
- **THEN** se emite `update:attendance_event` con `{user_id, status:"not_confirmed", source:null, registered_at:null, attendance_id:null}`.

#### Scenario: QR idempotente no emite

- **WHEN** un corredor registra asistencia por QR y ya la tenía (200 idempotente)
- **THEN** no se emite ningún evento.
