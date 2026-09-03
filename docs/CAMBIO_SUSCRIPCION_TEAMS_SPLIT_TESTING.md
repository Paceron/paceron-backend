# Pruebas de suscripción de equipos + split payments (suscripcion-teams-split) — Guía paso a paso

> Cómo probar de punta a punta el cobro de la mensualidad del corredor al entrenador con **split payments** de Mercado Pago (pago de prueba en sandbox): conexión OAuth del entrenador → crear equipo con `membership_fee` → dar de alta corredor (gate D2) → estado de cuenta → preferencia `team_subscription` con comisión de plataforma → pago → webhook → activación + cuota #2 → deudas y errores.

Flujo general (spec `suscripcion-teams-split`):

```
GET /mercadopago/connect  ──► URL OAuth ──► callback con code
      │                                    │
      │                                    ▼
      │                        exchange → seller_connections (token del entrenador)
      ▼
POST /payments/preference { concept=team_subscription, installment_id,
                            items[0].unit_price = cuota }   ──► marketplace_fee + token entrenador
      │
      ▼
Payment Brick / POST /payments { token, preference_id, installment_id }  ──► approved
      │
      ▼
Webhook /payments/webhook  ──► cuota paid + team_user active + cuota #2 generada
```

---

## Pre-requisitos

| Qué | Dónde | Nota |
|---|---|---|
| Backend corriendo (local con ngrok o staging de Render) | `.env` + `make run` o Render | Webhook y callback OAuth deben ser alcanzables por MP |
| App de Mercado Pago con OAuth habilitado | Panel MP → Tu app → OAuth | `MP_OAUTH_CLIENT_ID/SECRET/REDIRECT_URI` y `TOKEN_ENCRYPTION_KEY` en `.env` |
| Cuenta de prueba **vendedor** | Panel MP → Test users | Representa al entrenador que va a cobrar; es quien autoriza el OAuth |
| Test user de MP (payer) | Panel MP → Test users | El `payer_email` del pago debe ser un test user válido |
| Backend apuntando a sandbox | credenciales `TEST-...` | Ver `docs/PAYMENT_TESTING.md` |

### Variables del entorno nuevas

| Variable | Uso |
|---|---|
| `MP_OAUTH_CLIENT_ID` / `MP_OAUTH_CLIENT_SECRET` | Cliente de la app MP para el flujo OAuth mp-connect |
| `MP_OAUTH_REDIRECT_URI` | URI de callback (debe coincidir con la registrada en MP y con `GET /mercadopago/connect?redirect_uri=`) |
| `TOKEN_ENCRYPTION_KEY` | Clave AES (16/24/32 bytes) para cifrar los tokens de `seller_connections`; **no** se exponen en ningún response |

---

## Paso 1 — (Opcional) Configurar comisión de plataforma

`GET /api/v1/platform-settings/marketplace-fee` → devuelve el `marketplace_fee` actual (default `5.0`, porcentaje).

Para cambiarlo, `PUT /api/v1/platform-settings/marketplace-fee` con `{ "fee_percentage": 5.0 }`. La validación de rol de sistema (owner) está como TODO: hoy acepta cualquier usuario autenticado.

---

## Paso 2 — Conectar el entrenador (OAuth mp-connect)

Con un login de **entrenador**:

1. `GET /api/v1/mercadopago/connect` → devuelve `{ "url": "https://auth.mercadopago.com/authorization?client_id=...&redirect_uri=...&state=..." }`. El `state` es el CSRF (userID+timestamp).
2. Abrí esa `url` en el navegador, logueate con la cuenta de prueba **vendedor**, y autorizá. MP redirige a `/api/v1/mercadopago/connect/callback?code=...&state=...`.
3. El backend valida `state`, intercambia el `code` por tokens, guarda `seller_connections` (access/refresh cifrados).

**Qué verificar:**
- `GET /api/v1/mercadopago/connect/status` → `{ "connected": true, "account_status": "authorized" }`.
- En DB:
```sql
SELECT user_id, mp_user_id, status, token_expires_at
FROM seller_connections WHERE user_id = :entrenador_id;
-- status = authorized; access_token/refresh_token CIFRADOS (no legibles)
--   (NO los mostramos acá de propósito)
```

> Para simular el OAuth real en sandbox, la URL de autorización debe incluir `test_token` y usarse la cuenta de prueba **vendedor** (su access_token es `TEST-...`). Ver `docs/PAYMENT_TESTING.md`.

---

## Paso 3 — Seed: equipo con membresía paga

Crear un equipo con `membership_fee > 0`. Puede ser:

```sql
-- Actualizá un equipo existente (dueño = el entrenador conectado en el paso 2)
UPDATE teams SET membership_fee = 1500 WHERE id = :team_id AND owner_id = :entrenador_id;
```

O por API → `POST /api/v1/teams` queda con `membership_fee = 0` por defecto; es más cómodo matchear un equipo ya creado y actualizar su `membership_fee`. Confirmá:
```sql
SELECT id, name, owner_id, membership_fee FROM teams WHERE id = :team_id;
```

---

## Paso 4 — Dar de alta a un corredor (gate D2)

Con login del **entrenador**, o que el **corredor acepte una invitación**:

- **Alta directa:** `POST /api/v1/teams/:id/users` con el `user_id` del corredor y su rol.
- **Invitación:** `POST /api/v1/teams/:id/invite` (email) y luego el corredor acepta en `POST /api/v1/invitations/:id/accept`.

**Qué verificar (D2):** el `team_user` queda con `subscription_status = first_payment_pending` y se generó la **cuota #1** con `team_id`:

```sql
SELECT * FROM team_users WHERE team_id = :team_id AND user_id = :corredor_id;
-- subscription_status = first_payment_pending, init_amount = 1500, paid_installments = 0

SELECT * FROM installments WHERE team_id = :team_id AND user_id = :corredor_id
ORDER BY installment_number;
-- cuota #1: status = pending, amount = 1500, SIN due_date/blocked_date (la #1 nunca es deuda)
```

---

## Paso 5 — Estado de cuenta del equipo (D3)

`GET /api/v1/users/:corredor_id/teams/:team_id/subscription`

**Qué verificar:**
```json
{
  "team": { "id": 1, "name": "Equipo X", "membership_fee": 1500 },
  "membership": { "subscription_status": "first_payment_pending", "init_amount": 1500, "paid_installments": 0 },
  "next_installment": { "installment_number": 1, "installment_amount": 1500 },
  "has_debt": false,
  "mercadopago": { "public_key": "TEST-...", "concept": "team_subscription", "marketplace": true }
}
```
- La cuota #1 no genera `has_debt` (`false`).
- `mercadopago.marketplace = true` → el front arma un Brick de marketplace (paga al entrenador).
- Equipo gratis (`membership_fee = 0`): `subscription_status = "active"`, `next_installment = null`, `mercadopago = null`.

---

## Paso 6 — Crear preferencia con split

`POST /api/v1/payments/preference` con:
```json
{
  "concept": "team_subscription",
  "installment_id": <id de la cuota #1>,
  "items": [{ "title": "Membresía", "quantity": 1, "unit_price": 1500 }]
}
```

**Qué verificar:**
- El backend resuelve el split: token del dueño del equipo desde `seller_connections` (descifrado en memoria, nunca logueado) + `marketplace_fee = 5.0` desde `platform_settings`.
- `payments` queda con `concept = team_subscription`, `installment_id`, `seller_user_id = owner_id`, `marketplace_fee = 5.0`, `marketplace = true`.
```sql
SELECT id, concept, amount, status, installment_id, seller_user_id, marketplace_fee, marketplace
FROM payments ORDER BY id DESC LIMIT 1;
```

---

## Paso 7 — Generar token de tarjeta de prueba

Tarjeta Mastercard sandbox `5031 7557 3453 0604` → aprobada (deja `card_token`).

---

## Paso 8 — Procesar el pago

`POST /api/v1/payments` (formData del Brick: `token`, `payment_method_id`, `installment_id`, ...). **Qué verificar:** respuesta `status: "approved"` (el pago entra a la cuenta MP del **entrenador**, Paceron descuenta la comisión).

---

## Paso 9 — Webhook (confirmación y activación D10)

Dispará la notificación `payment` con el `payment_id` del paso 8 (simular desde MP o por Bruno).

**Qué verificar — activación completa:**
```sql
SELECT * FROM installments WHERE team_id = :team_id AND user_id = :corredor_id
ORDER BY installment_number;
-- cuota #1: status = paid, internal/external_payment_id ok
-- cuota #2: status = pending, due_date = hoy + 1 mes, blocked_date = due + 7 días

SELECT * FROM team_users WHERE team_id = :team_id AND user_id = :corredor_id;
-- subscription_status = active, paid_installments = 1
```
O por API: `GET .../teams/:team_id/subscription` → `subscription_status = "active"`, `paid_installments = 1`, y `next_installment` apunta a la **cuota #2** (con `next_due_date`/`blocked_date`).

### Idempotencia (doble notificación)
Corré el webhook de nuevo con el mismo `payment_id`. Nada debe cambiar (una sola cuota pendiente, `paid_installments = 1`).

---

## Paso 10 — Deuda y errores

### Deuda que bloquea dejar el equipo (D4)
Vencé la cuota pendiente actual:
```sql
UPDATE installments
SET blocked_date = NOW() - INTERVAL '1 day'
WHERE team_id = :team_id AND user_id = :corredor_id AND status = 'pending';
```
`DELETE /api/v1/teams/:team_id/users/:user_id` (corredor intenta la baja) → **debe fallar** con `code = "TEAM_DEBT_BLOCKS_OPERATION"` y mensaje tipo `"no podés dejar el equipo con deuda pendiente"`.

### Entrenador sin conectar
Creá otro equipo pago cuyo dueño NO tenga `seller_connections` (o la tenga `deauthorized`) y pedí una preferencia `team_subscription` → error con `code = "SELLER_NOT_CONNECTED"` (mensaje `"el entrenador debe conectar su cuenta de Mercado Pago"`).

### Comisión inválida
`PUT /platform-settings/marketplace-fee` con `{ "fee_percentage": 150 }` → `400` (debe estar entre 0 y 100).

---

## Verificación en DB — resumen

```sql
-- Conexión del entrenador
SELECT user_id, mp_user_id, status, token_expires_at FROM seller_connections;

-- Comisión configurada
SELECT * FROM platform_settings WHERE key = 'marketplace_fee_percentage';

-- Cuotas de la membresía de equipo
SELECT * FROM installments WHERE team_id = :team_id AND user_id = :corredor_id ORDER BY installment_number;

-- Membresía del corredor
SELECT subscription_status, init_amount, paid_installments FROM team_users
WHERE team_id = :team_id AND user_id = :corredor_id;

-- Pagos con split
SELECT id, concept, amount, status, installment_id, seller_user_id, marketplace_fee, marketplace
FROM payments WHERE concept = 'team_subscription' ORDER BY created_at DESC;
```

---

## Troubleshooting

| Síntoma | Causa probable | Solución |
|---|---|---|
| `SELLER_NOT_CONNECTED` al crear preferencia | El dueño del equipo no conectó MP (o está `deauthorized`) | Hacé el OAuth mp-connect del paso 2 con ese entrenador |
| Callback OAuth falla con `state inválido/`expirado`` | El `state` venció o pertenece a otro usuario | Volvé a pedir `GET /mercadopago/connect` y usá el `state` fresco |
| El `team_user` no queda `first_payment_pending` | `membership_fee` = 0 o el gate no corrió | Verificá `membership_fee > 0` en `teams` |
| El tier pago no se activa tras pagar | El webhook no llegó o la cuota ya estaba `paid` | Corré el webhook simulado; revisá logs (`applyApprovedTeamInstallment`) |
| `TEAM_DEBT_BLOCKS_OPERATION` al intentar salir sin causa | Quedó una cuota vencida de un intento anterior | Revisá cuotas `pending` vencidas del `team_user`; pagalas o limpiá el test |
| `422/400` de MP en el pago | Payer email no es un test user | Creá un test user en el panel MP |
| `marketplace_fee` no aparece en `payments` | `platform_settings` sin valor o fee = 0 | Seteá la comisión (paso 1); con fee = 0 el campo queda `null` |