## Why

Los corredores de un equipo pagan su mensualidad al **entrenador**, no a Paceron. Hoy no existe ningún mecanismo de cobro por equipo: `teams` no tiene monto mensual, `team_users` no guarda estado de suscripción, y los pagos solo cubren el flujo individual (`subscription` → Paceron). Además, para que el entrenador cobre hace falta la integración de Mercado Pago con **split payments** (el entrenador recibe y Paceron toma su comisión), que hoy no está: no hay conexión OAuth del entrenador (`seller_connections`) ni comisión configurable (`platform_settings`). Sin esto no se puede monetizar equipos ni controlar acceso por mensualidad al día.

## What Changes

- **Nueva columna** `teams.membership_fee` (numeric, default 0): monto mensual que paga cada corredor al entrenador. `0` = equipo gratis (sin suscripción).
- **`team_users` pasa a modelar la suscripción al equipo**: nuevas columnas `subscription_status` (`first_payment_pending` | `active`), `init_amount`, `paid_installments`. Unirse a un equipo (AddUser o aceptar invitación) con `membership_fee > 0` crea el `team_user` en `first_payment_pending` y genera la cuota #1 en `installments` con `team_id` (arco exclusivo definido en `cambio-tier-suscripciones`). El acceso pleno (`active`) recién se alcanza pagando la primera cuota.
- **Deuda bloqueante**: una cuota `pending` pasada su `blocked_date` impide dejar el equipo (`DELETE /api/v1/teams/:id/users/:user_id`).
- **Endpoint de estado de cuenta de equipo** `GET /api/v1/users/:id/teams/:team_id/subscription`: devuelve membresía + próxima cuota a pagar + `has_debt` + datos para el checkout Bricks con marketplace.
- **Split payments (Mercado Pago)**: nueva tabla `seller_connections` (OAuth mp-connect por entrenador, guarda su access_token y estado) y `platform_settings` (key-value; comisión de Paceron en porcentaje). Al pagar una cuota de equipo, el backend usa el token del entrenador (`team.owner_id`) y aplica `marketplace_fee`; `payments` registra `concept = team_subscription`, `seller_user_id` y `marketplace_fee` (columnas ya existentes).
- **Endpoints OAuth mp-connect**: `GET /mercadopago/connect` (URL de autorización), callback, `GET /mercadopago/connect/status` y webhook de desautorización; `GET/PUT /api/v1/platform-settings/marketplace-fee` (solo owner de la app).
- **Webhook de pago extendido**: al confirmarse una cuota de equipo, marca `paid`, incrementa `paid_installments` del `team_user`, activa la membresía si era cuota #1 y genera la cuota siguiente (reglas idénticas a `tier-subscriptions`, cambiando el contexto).

## Capabilities

### New Capabilities

- `team-subscriptions`: suscripción del corredor a un equipo (membresía gated por primer pago, `membership_fee` por equipo, cuotas en `installments` compartida, deuda que bloquea salida, endpoint de estado de cuenta `user/team`).
- `mercado-pago-split`: conexión OAuth del entrenador (`seller_connections`), comisión de Paceron (`platform_settings`), procesos de pago con el token del entrenador y `marketplace_fee`, endpoints OAuth y webhook mp-connect.

### Modified Capabilities

- `tier-subscriptions`: la tabla `installments` ya define la columna `team_id` (arco exclusivo) en el change `cambio-tier-suscripciones`; este change la utiliza. `payments.installment_id` también queda compartido.

## Impact

- **Schema/DB**: `teams.membership_fee`; `team_users.subscription_status`/`init_amount`/`paid_installments`; nuevas tablas `seller_connections` y `platform_settings`; `payments` ya tiene `marketplace_fee`/`seller_user_id` (se usan), `installment_id` viene del change anterior. AutoMigrate en `cmd/api/infrastructure/postgresdb/postgres.go`.
- **Modelos**: `cmd/api/domains/dbs/team.go`, `dbs/team_user.go`, `dbs/seller_connection.go`, `dbs/platform_setting.go`.
- **Dominios/DTOs**: nuevos `domains/teambio/` (estado de cuenta), `domains/mpconnect/`, `domains/platformsettings/`; ajustes en `domains/payment/` (concept `team_subscription`).
- **Servicios**: `services/team_user_service.go` (AddUser/RemoveUser con suscripción), `services/invitation_service.go` (AcceptInvitation), nuevos `services/team_subscription_service.go`, `services/mp_connect_service.go`, `services/platform_setting_service.go`, `services/payment_service.go` (webhook → cuota de equipo).
- **DAOs**: nuevos `daos/seller_connection_dao.go`, `daos/platform_setting_dao.go`; `daos/team_user_dao.go` y `daos/installment_dao.go` extienden.
- **Controllers/rutas**: `controllers/team_subscription_controller.go`, `controllers/mp_connect_controller.go`, `controllers/platform_setting_controller.go`; rutas en `cmd/api/app/url_mappings.go`.
- **API**: endpoints nuevos (estado de cuenta de equipo, OAuth mp-connect, platform-settings); `POST /api/v1/teams/:id/users`, `POST /api/v1/invitations/:id/accept` y `DELETE /api/v1/teams/:id/users/:user_id` cambian su comportamiento cuando el equipo cobra. **BREAKING** de comportamiento (no de contrato): miembros nuevos de equipos con `membership_fee > 0` no quedan `active` hasta el primer pago.
- **Integración**: Mercado Pago (Preference/Payment con token del entrenador + `marketplace_fee`, OAuth mp-connect, webhook); checkout Bricks **marketplace** en frontend.
- **Config**: credenciales de app mp-connect cliente (`MP_CLIENT_ID`, `MP_CLIENT_SECRET`, redirect_uri) en `.env.example` y config.
- **Swagger**: regenerar `cmd/api/docs`.
- **Seguridad**: el access_token del entrenador se persiste cifrado (o al menos nunca logueado); rotation flag en `seller_connections`.
- **Tests**: unitarios (services/controllers/daos con mocks) y DAO con Postgres real para los nuevos constraints.