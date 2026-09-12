# Team Configuration

## ADDED Requirements

### Requirement: Consultar configuración de equipo según tier del entrenador

El sistema SHALL exponer `GET /api/v1/team-configuration` (autenticado) que devuelva la configuración de creación/edición de equipos según el tier del entrenador, para usarse en el frontend al crear o editar un equipo.

- El endpoint SHALL requerir los query params `user_id` (id del entrenador) y `team_id` (id del equipo en cuestión); si faltan o no son numéricos SHALL responder `400`.
- La respuesta 200 SHALL ser `{"max_members": <int>, "minimum_fee": <float>}`.
- El tier SHALL resolverse sobre el rol `entrenador` del usuario con el mismo criterio que la suscripción vigente (`user_role_tier_subscriptions` active/first_payment_pending) y, en su ausencia, con `user_roles.tier_id`. El tier se matchea contra un mapa en código; un tier ausente del mapa o sin tier resuelto SHALL devolver el **default** `max_members = 10`, `minimum_fee = 20000`.
- El `user_id` autenticado SHALL ser el mismo que `user_id` (`403` en caso contrario).
- El equipo con `team_id` SHALL existir (`404` en caso contrario) y `teams.owner_id` SHALL ser igual a `user_id` (`403` en caso contrario).

#### Escenario: tier premium con sub activa
- **WHEN** el entrenador 7 (sub activa tier `premium`) consulta `user_id=7&team_id=3` siendo dueño del equipo 3
- **THEN** responde `200` con `{"max_members": 50, "minimum_fee": 20000}`

#### Escenario: tier base
- **WHEN** el entrenador dueño del equipo consulta su configuración con tier `base`
- **THEN** responde `200` con `{"max_members": 10, "minimum_fee": 20000}`

#### Escenario: tier desconocido o sin tier
- **WHEN** el entrenador dueño del equipo tiene un tier que no está en el mapa (o no tiene tier resuelto)
- **THEN** responde `200` con el default `{"max_members": 10, "minimum_fee": 20000}`

#### Escenario: params inválidos
- **WHEN** se llama sin `user_id`, sin `team_id`, o con valores no numéricos
- **THEN** responde `400`

#### Escenario: equipo inexistente
- **WHEN** `team_id` no corresponde a ningún equipo
- **THEN** responde `404` y no devuelve configuración

#### Escenario: usuario no dueño del equipo
- **WHEN** `user_id` no es el `owner_id` del equipo consultado
- **THEN** responde `403`

#### Escenario: user_id ajeno al autenticado
- **WHEN** el usuario autenticado consulta con un `user_id` distinto al suyo
- **THEN** responde `403`