# Team Configuration

## ADDED Requirements

### Requirement: Consultar configuración de equipo según tier del entrenador

El sistema SHALL exponer `GET /api/v1/team-configuration` (autenticado, sin query params) que devuelva la configuración de creación/edición de equipos según el tier del entrenador logueado, para usarse en el frontend al crear o editar un equipo.

- El endpoint SHALL tomar la identidad del **access token** (el usuario autenticado); no SHALL requerir `user_id` ni `team_id`.
- La respuesta 200 SHALL ser `{"max_members": <int>, "minimum_fee": <float>}`.
- El tier SHALL resolverse sobre el rol `entrenador` del usuario con el mismo criterio que la suscripción vigente (`user_role_tier_subscriptions` active/first_payment_pending) y, en su ausencia, con `user_roles.tier_id`. El tier se matchea contra un mapa en código; un tier ausente del mapa o sin tier resuelto SHALL devolver el **default** `max_members = 10`, `minimum_fee = 20000`.
- Sin identidad autenticada SHALL responder `401`.

#### Escenario: tier premium con sub activa
- **WHEN** el entrenador 7 (sub activa tier `premium`) autenticado consulta el endpoint
- **THEN** responde `200` con `{"max_members": 50, "minimum_fee": 20000}`

#### Escenario: tier base
- **WHEN** un entrenador autenticado con tier `base` consulta el endpoint
- **THEN** responde `200` con `{"max_members": 10, "minimum_fee": 20000}`

#### Escenario: tier desconocido o sin tier
- **WHEN** el entrenador autenticado tiene un tier que no está en el mapa (o no tiene tier resuelto)
- **THEN** responde `200` con el default `{"max_members": 10, "minimum_fee": 20000}`

#### Escenario: sin identidad autenticada
- **WHEN** se llama sin un access token válido
- **THEN** responde `401`