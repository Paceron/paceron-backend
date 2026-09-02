# Design: Suscripción de equipos y split payments

## Context

El modelo de suscripciones individuales (change `cambio-tier-suscripciones`) introduce `installments` con arco exclusivo (`subscription_id` XOR `team_id`). Este change usa la mitad de ese arco (`team_id`) para monetizar equipos: el corredor paga su mensualidad al **entrenador**, y Paceron retiene una comisión (split payment). Los entrenadores necesitan conectar su cuenta de Mercado Pago (OAuth mp-connect) para poder cobrar, y la comisión debe ser configurable por los dueños de la app.

## Goal

- Definir el monto mensual por equipo (`membership_fee`).
- Gate de membresía: unirse a un equipo pago exige primer pago; acceso pleno al pagar.
- Cuotas de equipo en la misma `installments` (historial financiero), con el mismo ciclo mensual y webhook idempotente que las de tier.
- Cobrar con split: el entrenador cobra y Paceron retiene `marketplace_fee`.
- Endpoints de estado de cuenta `user/team` y de conexión mp-connect.

## Non-Goals

- Cobros automáticos programados por el backend.
- Marketplace multivendedor: el "seller" de una cuota de equipo es siempre el dueño del equipo.
- Manejo de refunds/rembolsos de cuotas de equipo (queda como follow-up).
- Frontend (Vercel/Expo): este repo solo expone backend + payloads para Bricks `marketplace`.

## Decisions

### D1. `teams.membership_fee` + estado de suscripción en `team_users`

- `teams.membership_fee numeric NOT NULL DEFAULT 0` — mensualidad que paga cada corredor. `0` = gratis.
- `team_users` gana:
  - `subscription_status` (`first_payment_pending` | `active`; `ended` no hace falta porque al salir se baja la fila — ver D4)
  - `init_amount numeric` (monto de la cuota mensual = `membership_fee` al momento de unirse)
  - `paid_installments int DEFAULT 0`
- El `team_user` es la membresía actual; `installments` es el ledger (historial). El unique `(team_id, user_id)` existente garantiza una sola membresía/fila por par.

Se descartó una tabla `team_subscriptions` dedicada: no hay "cambio de tier" en un equipo (el contexto de cobro es el team), así que la membresía + cuotas alcanza y conserva el patrón de `user_roles.TierID` (el estado actual es el `team_users`; el dinero está en `installments`).

### D2. Gate de membresía (unirse requiere primer pago)

En `AddUser` (`POST /api/v1/teams/:id/users`) y `AcceptInvitation` (`POST /api/v1/invitations/:id/accept`), en una **transacción GORM**:
- `membership_fee == 0` → `team_user` con `subscription_status = active`, sin cuotas.
- `membership_fee > 0` → `team_user` con `subscription_status = first_payment_pending`, `init_amount = membership_fee`, `paid_installments = 0` + cuota #1 en `installments` con `team_id`, `installment_number = 1`, `status = pending`, `amount = membership_fee`, sin `due_date`/`blocked_date`.

El corredor tiene la fila de membresía pero con acceso pleno `active` recién cuando paga la cuota #1 (el frontend puede mostrar "pendiente de primer pago").

### D3. Estado de cuenta `user/team`

`GET /api/v1/users/:id/teams/:team_id/subscription`:

```json
{
  "team": { "id": 2, "name": "Los Pumas", "membership_fee": 5000.0 },
  "membership": {
    "subscription_status": "first_payment_pending",
    "init_amount": 5000.0,
    "paid_installments": 0,
    "start_date": "2026-09-02T12:00:00Z"
  },
  "next_installment": {
    "installment_id": 9,
    "installment_number": 1,
    "installment_amount": 5000.0,
    "next_due_date": null,
    "blocked_date": null
  },
  "has_debt": false,
  "mercadopago": {
    "public_key": "APP_USR-...",
    "concept": "team_subscription",
    "marketplace": true
  }
}
```

- Equipo gratis → `subscription_status = active`, `next_installment = null`, `mercadopago = null`.
- `has_debt` se calcula con la misma definición que tier (D5): cuota `pending` con `blocked_date`/`due_date` < hoy.

### D4. Deuda y salida del equipo

- `DELETE /api/v1/teams/:id/users/:user_id` se bloquea si el miembro tiene **deuda** (cuota `pending` pasada su `blocked_date` de su membresía vigente). Cuota #1 nunca es deuda.
- Sin deuda, se permite la baja: se **soft-delete** el `team_user` (conserva histórico) y las cuotas pendientes futuras dejan de contar (no hay membresía vigente). El historial financiero vive en `installments`.

Esto replica la regla anti-fraude de tier: no se puede "escapar" de una mensualidad vencida saliendo del equipo.

### D5. Deuda y cuotas de equipo (mismas reglas que tier)

Reutilizamos exactamente las reglas de `tier-subscriptions` cambiando el contexto:
- Deuda = cuota `pending` con `blocked_date` (o `due_date`) < now, de la membresía vigente.
- Al pagar la cuota `N` se genera la `N+1` (`due_date = start_date + 1 mes` si `N == 1`, si no `due_date` previa + 1 mes; `blocked_date = due_date + 7 días`).
- El webhook marca `paid` con update condicional (`status = pending`) → idempotencia.

La generación y el cálculo se centralizan en un servicio/helper compartido con tier-subscriptions (mismo "motor de cuotas"), parametrizado por contexto (padre = subscription o team).

### D6. `seller_connections` (OAuth mp-connect)

| columna | tipo | notas |
|---|---|---|
| `id` | PK | |
| `user_id` | int UNIQUE NOT NULL | entrenador |
| `mp_user_id` | string | id de la cuenta MP |
| `access_token` | text | **cifrado** con `infrastructure/crypto` (AES-GCM) — nunca logueado |
| `refresh_token` | text | **cifrado** igual que el access token (MP provee refresh token) |
| `token_expires_at` | timestamp NULL | para renovación/refresh automático |
| `status` | string | `authorized` / `deauthorized` |
| `created_at` / `updated_at` | | |

Solo usuarios con rol `entrenador` pueden conectar. Los tokens se usan server-side para crear preferencias/pagos con split y para refrescar la sesión OAuth; no se exponen en respuestas ni en logs.

### D7. Flujo OAuth mp-connect

- `GET /api/v1/mercadopago/connect?redirect_uri=...` → genera `state` (nonce aleatorio guardado en memoria/redis keyed por user), devuelve `{ url }` de MP (`authorization_url` con `response_type=code`, client_id, redirect_uri, state).
- Callback `GET /api/v1/mercadopago/connect/callback?code=&state=` → valida `state` contra el emitido, intercambia `code` por token (`oauth.Client.Create` de la SDK de MP), persiste/actualiza `seller_connections`, responde 200 (el frontend redirige donde corresponda).
- `GET /api/v1/mercadopago/connect/status` → `{ connected, account_status }`.
- Webhook `POST /api/v1/mercadopago/webhook/connect` (notificación de desautorización) → setea `status = deauthorized` (idempotente; MP reenvía).
- Las credenciales de app (client_id/secret) van a config: `MP_OAUTH_CLIENT_ID`, `MP_OAUTH_CLIENT_SECRET`, `MP_OAUTH_REDIRECT_URI`.

### D8. `platform_settings` y definición de "owner"

Tabla key-value genérica: `platform_settings (key TEXT PK, value JSONB, updated_by INT NULL, created_at, updated_at)`. Clave inicial: `marketplace_fee_percent` (default 5.0).

"Owner de la aplicación" = usuario con **rol `admin`** (rol de sistema ya existente: `corredor`, `entrenador`, `admin`). El `PUT /api/v1/platform-settings/marketplace-fee` valida que el usuario autenticado tenga una asignación con el rol `admin` (vía `roleDao.FindByName("admin")` + chequeo de `user_roles`).

### D9. Procesamiento de pagos con split

En `CreatePreference` y `ProcessPayment` cuando `concept = team_subscription`:
1. Resolver cuota por `installment_id` → `team_id`.
2. Cargar el team → `owner_id`.
3. Cargar `seller_connections` del owner; si no está `authorized` → error `SELLER_NOT_CONNECTED`.
4. `fee = redondeo(amount * marketplace_fee_percent / 100)`.
5. Llamar a MP con `access_token` del owner, `marketplace_fee = fee`, `marketplace = true`, sobre el precio pleno (`amount`).
6. Persistir `payments` con `concept = team_subscription`, `installment_id`, `seller_user_id = owner_id`, `marketplace_fee = fee`, `marketplace = true`.

`mpClient.CreatePreference`/`CreatePayment` ya aceptan `marketplaceFee` desde la iteración 1; se extiende para pasar también `marketplace=true` (el cliente MP calcula el split con su token de integrador). El token de la integración (Paceron) se usa solo en el flujo individual.

### D10. Webhook de pago extendido

El webhook `approved` resuelve el `payments` local (lógica existente) y, si `installment_id` está seteado, decide el contexto:
- `installment.subscription_id` → flujo tier (change `cambio-tier-suscripciones`).
- `installment.team_id` → flujo team: marcar `paid` (condicional), incrementar `paid_installments` del `team_user`, si era #1 activar membresía, generar cuota siguiente.

El split no se recalcula en el webhook: ya se aplicó al crear el pago.

### D11. Rutas nuevas

| Método | Ruta | Acción |
|---|---|---|
| POST | `/api/v1/teams/:id/users` | mod.: gate D2 |
| POST | `/api/v1/invitations/:id/accept` | mod.: gate D2 |
| DELETE | `/api/v1/teams/:id/users/:user_id` | mod.: bloqueo por deuda D4 |
| GET | `/api/v1/users/:id/teams/:team_id/subscription` | estado de cuenta D3 |
| GET | `/api/v1/mercadopago/connect` | URL de autorización |
| GET | `/api/v1/mercadopago/connect/callback` | exchange de code |
| GET | `/api/v1/mercadopago/connect/status` | estado de conexión |
| POST | `/api/v1/mercadopago/webhook/connect` | desautorización |
| GET | `/api/v1/platform-settings/marketplace-fee` | lectura comisión |
| PUT | `/api/v1/platform-settings/marketplace-fee` | escritura owner D8 |
| POST | `/api/v1/payments/preference` | mod.: split D9 |

## Open Questions

- **Cifrado de tokens**: se resuelve con un módulo `infrastructure/crypto` (AES-GCM) y clave desde config (`TOKEN_ENCRYPTION_KEY`); aplica a `access_token` y `refresh_token` de `seller_connections`.
- **UX de membresía pending**: decisión de frontend; el backend solo expone el estado (`first_payment_pending`/`active`).

## Migration

- AutoMigrate agrega `teams.membership_fee`, columnas de `team_users`, `seller_connections`, `platform_settings` — todas aditivas.
- `installments.team_id` y `payments.installment_id` ya existen desde `cambio-tier-suscripciones` (no se migra).
- Regenerar swagger.

## Follow-up

- Refunds de cuotas de equipo.
- Posible rol `admin` formal + backoffice.
- Pending task de rotación/renovación de token de seller (mitad OAuth): refresh automático antes de expirar si MP lo soporta.