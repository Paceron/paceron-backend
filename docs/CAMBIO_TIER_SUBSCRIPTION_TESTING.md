# Pruebas de suscripciones de tier (cambio-tier-suscripciones) — Guía paso a paso

> Cómo probar de punta a punta el cobro de cuotas de tier con un pago de pruebas (sandbox de Mercado Pago): asignación de rol pago → pago de la cuota #1 por webhook → activación del tier → cuota #2 generada → cambio de tier (con y sin deuda).

Flujo general del cambio (spec `cambio-tier-suscripciones`):

```
GET /users/:id/subscriptions/current  ──►  installment_id + amount + public_key
      │
      ▼
POST /payments/preference { installment_id, items }   (preference_id)
      │
      ▼
Payment Brick / POST /payments { token, preference_id, installment_id }  ──► approved
      │
      ▼
Webhook /payments/webhook  ──► cuota paid (idempotente) + sub active + tier sync + cuota #2
```

---

## Pre-requisitos

| Qué | Dónde | Nota |
|---|---|---|
| Backend corriendo (local con ngrok o staging de Render) | `.env` + `make run` o Render | Webhook debe ser alcanzable por MP |
| Colección Bruno `endpoint-collections/payments bruno collections/` | este repo | Incluye los 8 requests del flujo |
| Test user de MP (payer) | Panel MP → Test users | El `payer_email` del pago debe ser un test user válido |
| Producto/backend apuntando a sandbox | credenciales `TEST-...` | Ver `docs/PAYMENT_TESTING.md` para credenciales/webhook |

### Variables de la colección Bruno

Setear en la colección (Bruno → Collection → Variables) antes de arrancar:

| Variable | Valor | Se usa en |
|---|---|---|
| `token` | auto (request 1 — Login) | todos |
| `user_id` | el usuario que paga | 5, 6, 7, 8 |
| `role_id` | id del rol con tier pago | 5, 6, 8 |
| `tier_id` | id del tier pago target | 5 |
| `payer_email` | email del test user de MP | 8 |
| `card_token` | auto (request 3 — Obtener cardtkn) | 4, 8 |
| `preference` | auto (requests 2 y 7) | 4, 8 |
| `installment_id` / `installment_amount` | auto (request 6) | 7, 8 |
| `public_key` | auto (requests 2 y 7) | (referencia Bricks) |

---

## Paso 0 — Datos de prueba (seed)

El rol debe tener **dos tiers**: uno **gratis** (`payment_required=false`, jerarquía 1 = base) y uno **pago** (`payment_required=true` con `tier_amount`). Sin el tier gratis no puede asignarse el pago (AssignRole busca el tier de menor jerarquía para el acceso inicial).

Con Supabase testing (o la DB local):

```sql
-- Rol + tiers (ajustá role_name/id según tu ambiente)
INSERT INTO roles (name) VALUES ('corredor') RETURNING id;

INSERT INTO tiers (name, role_id, role_name, payment_required, tier_amount, hierarchy)
VALUES ('base', 1, 'corredor', false, 0, 1);

INSERT INTO tiers (name, role_id, role_name, payment_required, tier_amount, hierarchy)
VALUES ('premium', 1, 'corredor', true, 1500, 2);
```

Alternativa: usar la colección `bruno_coleccion_permisos/` (ya crea roles/tiers) y un usuario existente con `user_id`.

---

## Paso 1 — Login

Run **1 -Login - Obtener access token** de la colección. Deja `token` seteado (vence en 15 min; volver a correr si da 401).

## Paso 2 — Asignar rol con tier pago

`POST /api/v1/users/:id/roles` con `{ "role_id": <role_id>, "tier_id": <tier_id pago> }` (ej. `"tier_id": 4`).

**Qué verificar:**
- Status `201`.
- En DB `user_roles.tier_id` = **tier base** (el acceso arranca gratis, deja el pago para el pago de la cuota #1).

```sql
SELECT * FROM user_roles WHERE user_id = :user_id AND role_id = :role_id;
SELECT * FROM user_role_tier_subscriptions WHERE user_id = :user_id AND role_id = :role_id ORDER BY id DESC;
SELECT * FROM installments WHERE subscription_id = :sub_id ORDER BY installment_number;
```

Debe haber: sub en `first_payment_pending` y una cuota #1 `pending` **sin** `due_date`/`blocked_date` (la cuota #1 nunca genera deuda).

## Paso 3 — Consultar suscripción actual

Run **6 Consultar suscripcion actual**. La respuesta deja auto-seteado `installment_id` e `installment_amount`.

**Qué verificar:**
- `subscription_status = "first_payment_pending"`, `installment_id` y `installment_amount` presentes, `mercadopago.public_key` no vacío.

Si probás el lado gratis: con un rol sin tier pago responde solo `tier`/`role` (sin campos de cuota ni `mercadopago`).

## Paso 4 — Crear preferencia de la cuota

Run **7 Crear preferencia cuota suscripcion** (`concept=subscription` + `installment_id` + items con `unit_price = installment_amount`). Deja `preference` y `public_key` seteados.

> Los `items` son obligatorios (`binding:"required"`): el frente arma un item con el monto de la cuota obtenido del paso 3.

## Paso 5 — Generar token de tarjeta de prueba

Run **3 Obtener cardtkn** (tarjeta Mastercard sandbox `5031 7557 3453 0604` → aprobada). Deja `card_token` seteado.

## Paso 6 — Procesar el pago de la cuota

Run **8 Procesar pago cuota suscripcion** (incluye `installment_id`).

**Qué verificar:** respuesta `status: "approved"` y que el `payment_id` de MP queda impreso — lo vas a usar para el webhook.

## Paso 7 — Webhook (confirmación del pago)

Dos opciones:
- **Simular desde MP**: panel de tu app → Webhooks → **Simular notificación** → tipo `payment`, data id = `payment_id` del paso 6. (Cuando MP procesa el pago por `POST /payments`, la notificación debería llegar sola si `MERCADOPAGO_WEBHOOK_URL` es alcanzable con ngrok/Render.)
- **Bruno**: corré **04_webhook_simulacion.yml** reemplazando el `data.id` por el `payment_id` del paso 6.

**Qué verificar — activación completa (D6/D7):**

```sql
SELECT * FROM installments WHERE subscription_id = :sub_id ORDER BY installment_number;
-- cuota #1: status = paid, internal_payment_id = id del pago local, external_payment_id = payment_id de MP
-- cuota #2: status = pending, due_date = sub.start_date + 1 mes, blocked_date = due + 7 días

SELECT * FROM user_role_tier_subscriptions WHERE id = :sub_id;
-- status = active, paid_installments = 1

SELECT * FROM user_roles WHERE user_id = :user_id AND role_id = :role_id;
-- tier_id = TIER PAGO (sync D3)
```

O por API: run **6 Consultar suscripcion actual** → `subscription_status = "active"`, `paid_installments = 1`, e `installment_id` ahora apunta a la **cuota #2** (con `next_due_date` y `blocked_date`).

### Idempotencia (doble notificación)

Corré el webhook de nuevo (con el mismo `payment_id`). **Qué verificar:** nada cambia — seguís con una sola cuota pendiente (#2) y `paid_installments = 1` (el `MarkPaidConditional` deja de aplicar en la 2da vez). Si la doble notificación rompiera esto, la cuota #3 aparecería por error.

## Paso 8 — Cambio de tier (pago → gratis)

Run **5 Cambiar tier** con `tier_id` del tier gratis.

**Qué verificar (target gratis, D4):** status `200`, `subscription_status = "active"` (sin cuota nueva), y en DB `user_roles.tier_id` **ya apuntó al tier gratis de inmediato** (el sync gratis es directo, sin esperar pago).

## Paso 9 — Cambio de tier con deuda (bloqueo)

Generá una deuda a propósito: corré el cambio... no, primero creá la deuda manualmente en la sub activa del paso 8 o 7:

```sql
-- Tomá la cuota pendiente actual y vencé su bloqueo (deuda real)
UPDATE installments
SET blocked_date = NOW() - INTERVAL '1 day'
WHERE subscription_id = :sub_id AND status = 'pending';
```

Después run **5 Cambiar tier** a cualquier tier pago.

**Qué verificar:** status `409` con `code = "DEBT_BLOCKS_OPERATION"` y mensaje `"no podés cambiar de tier con deuda pendiente"`.

## Paso 10 — Otros casos de error

| Request | Resultado esperado |
|---|---|
| Cambiar tier a un tier de **otro rol** | `400`, `code = "TIER_ROLE_MISMATCH"` |
| Cambiar tier con `tier_id` inexistente | `404`, `code = "TIER_NOT_FOUND"` |
| Cambiar tier cuando la sub está `first_payment_pending` (cuota #1 sin pagar) | `409`, `code = "SUBSCRIPTION_PENDING_FIRST_PAYMENT"` |
| Cambiar tier de un rol no asignado | `404` |
| Consultar/editar de otro usuario (path `id` ≠ token) | `403` self-only |

---

## Verificación en DB — resumen

```sql
-- Sub y cuotas del usuario
SELECT s.id AS sub_id, s.status, s.paid_installments, s.tier_id, s.init_amount, s.start_date,
       i.installment_number, i.status AS ins_status, i.amount, i.due_date, i.blocked_date,
       i.internal_payment_id, i.external_payment_id
FROM user_role_tier_subscriptions s
JOIN installments i ON i.subscription_id = s.id
WHERE s.user_id = :user_id AND s.role_id = :role_id
ORDER BY i.installment_number;

-- Pago vinculado a la cuota
SELECT id, concept, amount, status, installment_id, payment_id
FROM payments
WHERE installment_id IS NOT NULL
ORDER BY created_at DESC;
```

---

## Troubleshooting

| Síntoma | Causa probable | Solución |
|---|---|---|
| `400` en preference | `items` vacío o `unit_price` mal | Mandá un item con `unit_price = installment_amount` |
| Webhook dice `payment not found locally` | El `data.id` del webhook no es un pago procesado contra este backend | Usá el `payment_id` real del paso 6 |
| `installment not found after marking paid` | `installment_id` mal en preference/process | Verificá que el `installment_id` del paso 3 se propagó hasta `payments.installment_id` |
| El tier no se activa tras pagar | El webhook no llegó o la cuota ya estaba `paid` | Corré el webhook simulado y revisá los logs (`applyApprovedInstallment`) |
| `409 DEBT_BLOCKS_OPERATION` inesperado | Quedó una cuota vencida de un intento anterior | Revisá `find pending installments`; pagala o limpiá el test |
| `422/400` de MP en el pago | Payer email no es un test user | Creá un test user en el panel MP |
| Cambio a gratis pero `user_roles.tier_id` no cambia | Corriste sobre una sub `first_payment_pending` (bloqueado) | Verificá que la sub esté `active` primero |