## ADDED Requirements

### Requirement: El sistema SHALL resolver el tier de cada rol usando la suscripción vigente del ledger

`GET /api/v1/auth/permissions` SHALL resolver el tier de cada rol del usuario consultando primero `user_role_tier_subscriptions` a través de `FindActiveByUserRole` (subs vigentes con status `active` o `first_payment_pending`). Si existe una sub vigente para el `(user_id, role_id)`, el `tier` reportado SHALL ser el `tier_id` de esa sub. Si no existe ninguna sub vigente, el sistema SHALL usar `user_roles.tier_id` como fallback.

#### Scenario: Sub vigente activa con tier distinto al campo denormalizado de user_roles
- **WHEN** un usuario tiene una sub `active` cuyo `tier_id` corresponde al tier base del rol mientras que `user_roles.tier_id` apunta a un tier pago del mismo rol
- **THEN** el endpoint reporta `tier` con el nombre del tier base de la sub vigente

#### Scenario: Sub vigente con primer pago pendiente
- **WHEN** la única sub vigente del `(user_id, role_id)` está en `first_payment_pending` hacia un tier pago
- **THEN** el endpoint reporta `tier` con el nombre del tier pago de la sub vigente

#### Scenario: Sin suscripción vigente usa el tier de la asignación
- **WHEN** no existe ninguna sub vigente (`active` ni `first_payment_pending`) para el `(user_id, role_id)` pero el rol está asignado en `user_roles`
- **THEN** el endpoint reporta `tier` con el nombre del tier apuntado por `user_roles.tier_id`

### Requirement: El sistema SHALL mantener el contrato de datos faltantes con el tier resuelto

Cuando el tier resuelto (desde la sub vigente o desde el fallback de `user_roles.tier_id`) no existe en el catálogo de tiers, el sistema SHALL registrar el problema y responder con error de datos faltantes, igual que hoy. La respuesta exitosa SHALL mantener la misma forma JSON (`user_id`, `roles[].{id, name, tier, permissions}`) sin importar el origen del tier.

#### Scenario: El tier de la sub vigente no está configurado
- **WHEN** la sub vigente apunta a un `tier_id` que no existe en el catálogo de tiers
- **THEN** el endpoint responde con un error de datos faltantes indicando el `tier_id` no configurado para el rol

#### Scenario: El tier de fallback no está configurado
- **WHEN** no hay sub vigente y el `user_roles.tier_id` apunta a un `tier_id` que no existe en el catálogo de tiers
- **THEN** el endpoint responde con un error de datos faltantes indicando el `tier_id` no configurado para el rol

#### Scenario: Respuesta exitosa mantiene la forma del contrato
- **WHEN** la resolución de tier es exitosa (por sub vigente o por fallback)
- **THEN** la respuesta JSON conserva exactamente los campos `user_id` y `roles` con `id`, `name`, `tier` y `permissions`, sin campos nuevos ni renombrados