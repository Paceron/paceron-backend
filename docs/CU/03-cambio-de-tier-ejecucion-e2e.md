# Validación end-to-end: Cambio de tier (ejecución real)

> Prueba **end-to-end** del flujo de cambio de tier del CU `01-cambio-de-tier.md`,
> corrida contra el **backend de testing** (`localhost:8080`, stage Supabase
> testing) con datos **reales** del entorno no vacío. Documento con los
> **requests y responses concretos** de cada llamada, tal como los ve el front
> (Expo / React Native), para que sirva de evidencia y de referencia de qué
> devuelve el backend hoy.
>
> **Resultado: ✅ TODO VERDE.** El flujo completo (login → descubrir → cambiar/pagar
> → webhook → verificar) terminó con la suscripción `active` y el tier `premium_corredor`
> activado. La corrida destapó además un **bug real** (`payment_id` no persistido)
> que se arregló antes de repetir la prueba — ver sección [Bug encontrado y fix](#bug-encontrado-y-fix).

---

## Datos del contexto de la prueba

- **Usuario autenticado:** `pepa@lota.com` / `Abcd-12345` → `user_id = 3` (rol `corredor`).
- **Rol a subir:** `role_id = 1` (`corredor`), tier actual `base_corredor`.
- **Tier objetivo (pago):** `premium_corredor`, `id = 4`, `tier_amount = $50000`, `payment_required = true`.
- **Tarjeta de prueba (MP sandbox):** Mastercard `5031 7557 3453 0604` (pago aprobado).
- **`payer_email` usado:** `andres.paez.teran@gmail.com`.

> **Nota importante — estado previo:** al re-correr la prueba, la cuenta **ya tenía
> una cuota #1 pendiente** de una corrida anterior. Por eso el paso 4 devolvió
> `409 SUBSCRIPTION_PENDING_FIRST_PAYMENT` en lugar de crear una cuota nueva
> (comportamiento correcto y documentado). El flujo continuó pagando esa cuota
> pendiente en lugar de intentar cambiar de tier de nuevo.

---

## Paso a paso con datos reales

### Paso 1 — Login

**Request**

```
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "pepa@lota.com",
  "password": "Abcd-12345"
}
```

**Response (200)**

```json
{
  "access_token": "<JWT>",
  "refresh_token": "lJln8sA9X3OLYsaGBRRLXX_htg3bPlUFgAl4QNtvtD0",
  "expires_in": 900,
  "user": {
    "user_id": 3,
    "name": "pepe",
    "surname": "lota",
    "email": "pepa@lota.com",
    "dni": "33703637",
    "birth_date": "31/12/1987",
    "status": "active"
  }
}
```

**Capturas:** `token = access_token`, `user_id = 3`.

---

### Paso 2 — Descubrir mis roles y mi tier actual

**Request**

```
GET /api/v1/auth/permissions?user_id=3
Authorization: Bearer {token}
```

**Response (200)**

```json
{
  "user_id": 3,
  "roles": [
    { "id": 1, "name": "corredor", "tier": "base_corredor", "permissions": ["crear_equipos"] }
  ]
}
```

**Capturas:** `role_id = 1`, tier actual `base_corredor`.

> Nótese que el `tier` viene como nombre compuesto (`base_corredor`), por la regla
> D11 que detecta la jerarquía por la **primera palabra** del nombre.

---

### Paso 3 — Descubrir los tiers disponibles del rol

**Request**

```
GET /api/v1/tiers?role_id=1
Authorization: Bearer {token}
```

**Response (200)** — el campo `hierarchy` viene poblado (ajuste de la rama feature).

```json
[
  {
    "id": 1,
    "name": "base_corredor",
    "description": "Tier base para corredor",
    "role_id": 1,
    "role_name": "corredor",
    "payment_required": false,
    "tier_amount": 0,
    "hierarchy": 1,
    "created_at": "2026-07-18T22:26:35.899429-03:00",
    "updated_at": "2026-07-18T22:26:35.899429-03:00"
  },
  {
    "id": 4,
    "name": "premium_corredor",
    "description": "Tier premium para corredor",
    "role_id": 1,
    "role_name": "corredor",
    "payment_required": true,
    "tier_amount": 50000,
    "hierarchy": 2,
    "created_at": "2026-07-18T22:30:45.050784-03:00",
    "updated_at": "2026-07-18T22:30:45.050784-03:00"
  }
]
```

**Capturas:** tier objetivo `id = 4` (`premium_corredor`, pago), `tier_amount = 50000`.

---

### Paso 4 — Cambiar de tier

Como había una cuota #1 pendiente de una corrida anterior, el backend **rechazó el
cambio** (correcto).

**Request**

```
PUT /api/v1/users/3/roles/1/tier
Authorization: Bearer {token}

{
  "tier_id": 4
}
```

**Response (409)** — el front debe llevar al usuario a pagar la cuota pendiente.

```json
{
  "status_code": 409,
  "code": "SUBSCRIPTION_PENDING_FIRST_PAYMENT",
  "message": "no podés cambiar de tier con el primer pago pendiente"
}
```

---

### Paso 5 — Leer la suscripción actual (la cuota pendiente a pagar)

**Request**

```
GET /api/v1/users/3/subscriptions/current?role_id=1
Authorization: Bearer {token}
```

**Response (200)** — la cuota #1 pendiente ($50000) y la `public_key` para el Bricks.

```json
{
  "subscription_id": 1,
  "subscription_status": "first_payment_pending",
  "installment_id": 1,
  "installment_number": 1,
  "installment_amount": 50000,
  "next_due_date": null,
  "blocked_date": null,
  "paid_installments": 0,
  "tier": { "id": 4, "name": "premium_corredor", "hierarchy": 2, "payment_required": true },
  "role": { "id": 1, "name": "corredor" },
  "mercadopago": { "public_key": "TEST-9a1e5d38-929e-45f8-ad90-01ca7887fe82" }
}
```

**Capturas:** `installment_id = 1`, `installment_amount = 50000`, `public_key`.

---

### Paso 6 — Crear la preferencia de la cuota

**Request**

```
POST /api/v1/payments/preference
Authorization: Bearer {token}

{
  "concept": "subscription",
  "description": "Cuota de suscripcion de tier",
  "items": [ { "title": "Cuota mensual tier", "quantity": 1, "unit_price": 50000 } ],
  "installment_id": 1
}
```

**Response (201)**

```json
{
  "preference_id": "40671376-9863be16-3db6-44a8-a932-cc7d640f07fe",
  "public_key": "TEST-9a1e5d38-929e-45f8-ad90-01ca7887fe82"
}
```

**Capturas:** `preference_id`.

---

### Paso 7 — Obtener token de tarjeta (sandbox)

**Request**

```
POST /api/v1/payments/test-card-token
Authorization: Bearer {token}

{
  "card_number": "5031755734530604",
  "expiration_month": "11",
  "expiration_year": "2030",
  "security_code": "123",
  "cardholder_name": "APRO Test User",
  "identification_type": "DNI",
  "identification_number": "12345678"
}
```

**Response (200)**

```json
{ "token": "7fc0bcfa70ebf6f97db0c9d9f3451aa4" }
```

**Capturas:** `card_token`.

---

### Paso 8 — Procesar el pago de la cuota

**Request**

```
POST /api/v1/payments
Authorization: Bearer {token}

{
  "token": "7fc0bcfa70ebf6f97db0c9d9f3451aa4",
  "transaction_amount": 50000,
  "payment_method_id": "master",
  "installments": 1,
  "payer_email": "andres.paez.teran@gmail.com",
  "preference_id": "40671376-9863be16-3db6-44a8-a932-cc7d640f07fe",
  "installment_id": 1
}
```

**Response (200)** — MP aprueba el pago.

```json
{
  "id": 49,
  "preference_id": "40671376-9863be16-3db6-44a8-a932-cc7d640f07fe",
  "payment_id": "1328049952",
  "external_reference": "",
  "concept": "order",
  "description": "",
  "amount": 50000,
  "currency_id": "ARS",
  "status": "approved",
  "status_detail": "accredited",
  "payment_method_id": "master",
  "installments": 1,
  "payer_email": "andres.paez.teran@gmail.com",
  "created_at": "2026-09-04T00:54:37-03:00"
}
```

**Capturas:** `payment.payment_id = 1328049952` (id de MP), `payment.id = 49` (id local).

> **Check del fix:** `payer_email` usado fue un email real (`andres.paez.teran@gmail.com`)
> y la API de MP en sandbox lo aceptó como payer. Para producción el MD recomienda
> un `test_user_*`.

---

### Paso 9 — Confirmar el pago (webhook MP, simulado)

> En sandbox el front simula la notificación con el request opcional de la colección.

**Request**

```
POST /api/v1/payments/webhook
X-Signature: dummy-signature-for-testing
X-Request-Id: test-request-123
Content-Type: application/json

{
  "id": 1350795217,
  "live_mode": false,
  "type": "payment",
  "action": "payment.created",
  "data": { "id": "1328049952" }
}
```

**Response (200)** — `"ok"` (el backend confirmó la cuota y avanzó el ciclo).

```json
{ "message": "ok" }
```

---

### Paso 10 — Verificar el upgrade (suscripción activa en el nuevo tier)

**Request**

```
GET /api/v1/users/3/subscriptions/current?role_id=1
Authorization: Bearer {token}
```

**Response (200)** — ✅ upgrade confirmado.

```json
{
  "subscription_id": 1,
  "subscription_status": "active",
  "installment_id": 2,
  "installment_number": 2,
  "installment_amount": 50000,
  "next_due_date": "2026-10-04T00:07:28.64971-03:00",
  "blocked_date": "2026-10-11T00:07:28.64971-03:00",
  "paid_installments": 1,
  "tier": { "id": 4, "name": "premium_corredor", "hierarchy": 2, "payment_required": true },
  "role": { "id": 1, "name": "corredor" },
  "mercadopago": { "public_key": "TEST-9a1e5d38-929e-45f8-ad90-01ca7887fe82" }
}
```

---

## Verificaciones de éxito

| Criterio | Antes | Después | ✅ |
|---|---|---|---|
| `subscription_status` | `first_payment_pending` | `active` | ✅ |
| `paid_installments` | `0` | `1` | ✅ |
| `tier.name` | `base_corredor` | `premium_corredor` | ✅ |
| `payment_id` del pago | `""` (persistía vacío) | `"1328049952"` | ✅ |
| cuota #2 (`installment_id`) | — | `2` con `next_due_date` y `blocked_date` | ✅ |
| webhook | `"error processing"` | `"ok"` | ✅ |

---

## Bug encontrado y fix

**Síntoma (primera corrida):** el pago se aprobaba en MP (`status: approved`) pero el
tier **nunca se activaba**: la suscripción seguía en `first_payment_pending`, el
webhook devolvía `{"message":"error processing"}` y `GET /payments/{id}` mostraba
`payment_id: ""`.

**Causa raíz:** en `payment_service.ProcessPayment` el registro se insertaba en la DB
**antes** de llamar a `CreatePayment` de Mercado Pago, y el `payment_id` devuelto por
MP se seteaba **solo en memoria** (nunca se persistía). El webhook hacía
`FindByPaymentID($mpPaymentID)` y no encontraba el pago local → la cuota no se
confirmaba y el tier no avanzaba.

**Fix (`fix/persist-mp-payment-id`):**
- `payment_dao.go`: nuevo `UpdatePaymentID(ctx, paymentID, mpPaymentID)` (persiste la columna `payment_id`).
- `payment_service.go`: tras `CreatePayment`, se persiste el `payment_id` de MP antes del resto de updates.
- Tests: expectativas nuevas en los 4 tests de éxito de `ProcessPayment` + 2 tests DB-backed del DAO.

Tras el fix, la **re-corrida completa salió verde** (la de este documento).

---

## Notas para el front

- El `GET /tiers?role_id=` es el camino para descubrir el `tier_id` pago y su monto; el
  campo `hierarchy` ya viene en la respuesta.
- Si `ChangeTier` responde `409 SUBSCRIPTION_PENDING_FIRST_PAYMENT`, el front debe
  **llevar al usuario a pagar la cuota pendiente** (que obtiene de
  `subscriptions/current`), no reintentar el cambio a ciegas.
- El `payment_id` del pago ahora se persiste, con lo que el **polling**
  (`GET /payments/{id}`) y el **webhook** pueden confirmar y activar el tier.
- Quirk conocido: en la respuesta de `POST /payments`, el `concept` devuelve `"order"`
  y `description` `""` aunque el request mande `subscription`/descripción (se persiste
  correcto para el webhook, pero el objeto de respuesta los muestra vacíos).