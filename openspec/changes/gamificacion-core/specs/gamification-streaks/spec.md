# Spec Delta — gamification-streaks

## ADDED Requirements

### Requirement: Consulta de rachas del corredor

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /users/{id}/streaks` disponible; el usuario autenticado solo puede ver sus propias
  rachas (`{id}` distinto del caller responde `403`).
- Query param `days` opcional; default `42`, tope `365`; fuera de rango (`< 1` o `> 365`)
  responde `400`.
- La ventana de cálculo va desde `hoy - (days - 1)` hasta AYER inclusive, en TZ
  `America/Argentina/Cordoba` fija. El día de hoy NUNCA corta una racha.
- Cálculo en batch, sin N+1: 3 queries totales (memberships activos del usuario, días de
  calendario `kind='training'` en el rango por `group_id IN`, runner sessions `finished` del
  rango por `session_instance_id IN`), más el mapeo por día en Go.
- Cada día aparece con `strict_status` y `flexible_status`, cada uno
  `completed|failed|skipped|pending`:
  - strict: training asignado y TODAS sus sesiones finished → `completed` (los salteados no
    importan); alguna training sin finished → `failed`; rest/other/cancelled o día sin
    asignación → `skipped`.
  - flexible: al menos una finished → `completed`; ninguna finished con training asignado →
    `failed`; rest/other/cancelled o sin training → `skipped`.
- El día de hoy, cuando el rango lo incluye, se emite con estado `pending` (en curso) y no
  corta ni rompe rachas.
- Racha estricta (A): corre con días completed, corta con cualquier failed; skipped NO corta.
  Racha flexible (B): corre con completed, corta con failed.
- `current` y `best` de ambas rachas se computan sobre TODO el histórico hasta ayer: la
  ventana `days` solo recorta el array de días devuelto; best NO es local a la ventana.
- Respuesta `200`: `{strict: {current, best}, flexible: {current, best},
  days: [{date, strict_status, flexible_status}], has_unseen}` (has_unseen de la sección
  streaks).

#### Scenario: Lectura propia con ventana default

- **WHEN** el corredor llama a `GET /users/su-id/streaks` sin `days`
- **THEN** responde `200` con la ventana de 42 días (de 41 días atrás hasta el día de
  hoy inclusive, con el día de hoy marcado `pending`) y `current` y `best` calculados sobre
  todo el histórico hasta ayer (el día de hoy no los modifica).
- **AND** los tres queries de lectura (memberships, calendario, sesiones) corren en batch, sin
  una query por día.

#### Scenario: days fuera de rango

- **WHEN** se envía `days=0` o `days=366`
- **THEN** responde `400` sin ejecutar la consulta.

#### Scenario: El día de hoy no corta

- **WHEN** ayer el corredor cumplió su día y hoy todavía no entrenó
- **THEN** la respuesta lo muestra con `current` igual a la racha acumulada hasta ayer
- **AND** el día de hoy aparece con estado `pending` en el array de días.

#### Scenario: Día skipped no corta la racha estricta

- **WHEN** en medio de una racha estricta hay un día `rest` sin entrenamiento
- **THEN** ese día queda `skipped` y la racha estricta sigue acumulando.

#### Scenario: Día failed corta la racha estricta

- **WHEN** en medio de una racha estricta hay un día con training asignado y su sesión quedó
  `interrupted` (sin finished)
- **THEN** ese día queda `failed` en strict_status y la racha estricta se corta en ese día.

#### Scenario: Racha flexible con sesiones parciales

- **WHEN** un día tiene el training asignado, tres sesiones y solo una quedó finished
- **THEN** ese día queda `completed` en flexible_status.
- **AND** el mismo día queda `failed` en strict_status.

#### Scenario: Solo el array se recorta por la ventana

- **WHEN** el corredor histórico tiene una racha de 60 días y pide `days=7`
- **THEN** `days` trae 7 elementos (de `hoy-6` hasta el día de hoy inclusive, con el día
  de hoy marcado `pending`) pero `current` y `best` reflejan el histórico completo hasta
  ayer, no solo la ventana.

#### Scenario: Rachas ajenas

- **WHEN** el usuario A consulta `GET /users/{id de B}/streaks`
- **THEN** responde `403` sin tocar el DAO.

### Requirement: Determinismo temporal

El sistema DEBE cumplir lo siguiente (MUST):

- La timezone y el "hoy" de gamificación se resuelven en un helper de paquete con variable
  global fija en producción (`America/Argentina/Cordoba`) y sobreescribible en tests.
- Todos los cálculos de frontera de día (ventana, ayer, hoy) usan ese helper; ningún cálculo
  de gamificación llama `time.Now()` directo para derivar el día.
- Los tests de rachas fijan el "hoy" con `time.Date` para que no dependan de la hora real del
  runner de CI.

#### Scenario: Test determinista con fecha fija

- **WHEN** un test fija la variable global del helper con `time.Date` y corre a cualquier hora
  real
- **THEN** el resultado del GET es el mismo a cualquier horario real del día del runner.

#### Scenario: Ventana compuesta a medianoche en Cordoba

- **WHEN** el helper fija hoy en Cordoba y la hora real del servidor en UTC es posterior
- **THEN** la ventana responde a la fecha de Cordoba, no a la del servidor.

#### Scenario: historia previa a la membresia no corta la racha

- **WHEN** un corredor se une a un grupo con historico de calendar de trainings previos a su date_start
- **THEN** los dias de training anteriores al ingreso del corredor en ESE grupo se clasifican `skipped` (no `failed`) y no cortan su racha
- **AND** la racha current/best se consolida solo desde el dia de alta inclusive
