# Caso de uso: Haciendo un cambio de tier

> El corredor quiere **subir de tier** en una suscripción que ya tiene activa
> (pasar de un tier **gratis/base** a un tier **pago, p.ej. premium**), pagar su
> cuota de suscripción y que la cuenta quede activa con el nuevo tier.
>
> Este doc es el **paso a paso de endpoints que el front (React Native / Expo)
> debe ejecutar** para completar el flujo de punta a punta, incluyendo todo lo
> que el front debe **descubrir primero** (no conoce de antemano el `role_id` ni
> el `tier_id`) más la obtención del **token de tarjeta** y el **pago** de la
> cuota.
>
> Cada llamada referenciada acá tiene su request en la colección de Bruno
> `endpoint-collections/CU cambio de tier/` (mismo orden/numeración).

---

## Contexto

- El usuario ya existe y **ya tiene asignado un rol** con un tier **base (gratis)** y la suscripción está **activa**.
- Quiere pasar a un nivel superior **pago** de **ese mismo rol**.
- El front **NO conoce** los ids: ni el id del usuario, ni el `role_id`, ni el `tier_id` del plan al que quiere subir. Los tiene que **conseguir por API**.
- Al subir a un tier pago, se crea automáticamente una **suscripción** con estado `first_payment_pending` y una **cuota #1** a pagar. Mientras no se pague, el acceso sigue en el tier base (el tier del rol recién cambia cuando la cuota se paga).

---

## Mapa del flujo (vista rápida)

```
                              ┌─▶ (lectura) roles + tier actual
 1. Login ────────────────────┘
      │
      ▼
 2. Descubrir MIS roles/tier   GET /auth/permissions        → role_id
      ▼
 3. Descubrir tiers del rol    GET /tiers (filter role_id)  → tier_id + monto
      ▼
 4. Cambiar de tier            PUT /users/{id}/roles/{role_id}/tier
      │                        → crea sub first_payment_pending + cuota #1
      ▼
 5. Leer suscripción           GET /users/{id}/subscriptions/current
      │                        → installment_id + installment_amount + public_key (presencia Bricks)
      ▼
 6. Crear preferencia          POST /payments/preference    → preference_id
      ▼
 7. Obtener token de tarjeta   (Bricks Card Form en front) / POST /payments/test-card-token
      ▼
 8. Procesar el pago           POST /payments               → status approved
      ▼
 9. (MP) confirmar por webhook /a la cuota se marca paga, el tier se activa
      ▼
10. Verificar el upgrade        GET /users/{id}/subscriptions/current → active + tier premium
```

---

## Paso a paso detallado

> **Nota de autenticación:** salvo el login, todas las llamadas llevan el header
> `Authorization: Bearer {access_token}`.

### Paso 1 — Login (obtener sesión)

El front reconoce al usuario (tenés su email/password) y obtiene los tokens y su `user_id`.

**Request**

```
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "corredor@example.com",
  "password": "Abcd-12345"
}
```

**Respuesta (200)**

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_in": 900,
  "user": { "user_id": 42, "email": "corredor@example.com", "name": "Juan", "..." : "..." }
}
```

**Qué capturamos para el resto del flujo**

| Variable | Valor |
|---|---|
| `token` | `access_token` |
| `user_id` | `user.user_id` (43 → `42`) |

> El access token vence en 15 min (`expires_in`). Si tirás `401` más adelante,
> renovalo con `POST /api/v1/auth/refresh`.

---

### Paso 2 — Descubrir mis roles y mi tier actual (lectura)

El front quiere saber **qué rol** va a subir y su **tier actual**. No lo tiene guardado; lo pregunta.

**Request**

```
GET /api/v1/auth/permissions?user_id=42
Authorization: Bearer {token}
```

**Respuesta (200)**

```json
{
  "user_id": 42,
  "roles": [
    {
      "id": 3,
      "name": "corredor",
      "tier": "base",
      "permissions": ["ver_perfil", "ver_eventos"]
    }
  ]
}
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `role_id` | `roles[0].id` → `3` (el rol que queremos subir) |
| tier actual | `roles[0].tier` → `"base"` (confirmamos que hoy está en base) |

> `tier` acá viene como el **nombre** corto del tier. El **id** del tier target lo
> averiguamos en el paso siguiente, porque el `ChangeTier` necesita el `tier_id`.

---

### Paso 3 — Descubrir los tiers disponibles de ese rol (lectura)

El front necesita el **id del tier pago** al que va a subir, y la **colección** de tiers del rol para no mandar un tier de otro rol (el backend lo rechazaría con `TIER_ROLE_MISMATCH`).

**Request**

```
GET /api/v1/tiers
Authorization: Bearer {token}
```

**Respuesta (200)** — array de tiers (filtramos por `role_id == 3`):

```json
[
  { "id": 10, "name": "base",    "role_id": 3, "role_name": "corredor", "payment_required": false, "tier_amount": 0 },
  { "id": 11, "name": "premium", "role_id": 3, "role_name": "corredor", "payment_required": true,  "tier_amount": 1500 }
]
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `tier_id` | el id del tier pago target → `11` ("premium", `payment_required=true`) |
| `tier_amount` | `1500` → monto que se va a cobrar como cuota #1 |

> Filtro recomendado: `tiers.filter(t => t.role_id === role_id && t.payment_required)`
> para quedarte con los planes pagos de ese rol.

---

### Paso 4 — Cambiar de tier (el corazón del caso de uso)

Le decimos al backend que el rol `3` del usuario `42` pasa al tier `11`. Como es un tier **pago**, el backend:
- cierra la suscripción vigente (base),
- crea la **nueva suscripción** en `first_payment_pending` con su monto y
- genera la **cuota #1** lista para pagar.

**Request**

```
PUT /api/v1/users/42/roles/3/tier
Authorization: Bearer {token}

{
  "tier_id": 11
}
```

**Respuesta (200)** — ya incluye la cuota a pagar y la `public_key` para el Bricks:

```json
{
  "subscription_id": 77,
  "subscription_status": "first_payment_pending",
  "installment_id": 501,
  "installment_number": 1,
  "installment_amount": 1500,
  "next_due_date": null,
  "blocked_date": null,
  "paid_installments": 0,
  "tier": { "id": 11, "name": "premium", "hierarchy": 2, "payment_required": true },
  "role": { "id": 3, "name": "corredor" },
  "mercadopago": { "public_key": "APP_USR-..." }
}
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `installment_id` | `501` |
| `installment_amount` | `1500` |
| `public_key` | `APP_USR-...` (para inicializar el Bricks de pago) |

> El tier del rol **todavía es "base"** a nivel de acceso (`user_roles.tier_id` se
> conserva hasta pagar la cuota #1). Recién cuando la cuota se paga, el acceso
> pasa a premium.

**Errores esperados** (el backend devuelve `code` tipificado):

| Código | Status | Significado | Acción del front |
|---|---|---|---|
| `TIER_NOT_FOUND` | 404 | el tier id no existe | mostrar error / recargar lista |
| `TIER_ROLE_MISMATCH` | 400 | tier de otro rol | elegir un tier del rol correcto |
| `SUBSCRIPTION_PENDING_FIRST_PAYMENT` | 409 | ya hay una cuota #1 sin pagar | llevá al usuario a pagar esa cuota |
| `DEBT_BLOCKS_OPERATION` | 409 | hay deuda vencida | pagar deuda antes de subir |

---

### Paso 5 — Leer la suscripción actual (confirmar monto + presencia Bricks)

Consulta canónica de la suscripción del rol: devuelve la **cuota a pagar** y la `public_key`. Sirve para (a) confirmar lo que devolvió el paso 4 y (b) tener todo lo necesario para inicializar el Bricks en el front.

**Request**

```
GET /api/v1/users/42/subscriptions/current?role_id=3
Authorization: Bearer {token}
```

**Respuesta (200)**

```json
{
  "subscription_id": 77,
  "subscription_status": "first_payment_pending",
  "installment_id": 501,
  "installment_number": 1,
  "installment_amount": 1500,
  "next_due_date": null,
  "blocked_date": null,
  "paid_installments": 0,
  "tier": { "id": 11, "name": "premium", "hierarchy": 2, "payment_required": true },
  "role": { "id": 3, "name": "corredor" },
  "mercadopago": { "public_key": "APP_USR-..." }
}
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `installment_id` | `501` (id de la cuota que vamos a pagar) |
| `installment_amount` | `1500` (monto a pagar) |
| `public_key` | `APP_USR-...` (usar para el Bricks) |

> La `public_key` es la que el front usa para **inicializar la presencia del
> Payment Brick** (Card Form) en pantalla: `new MercadoPago({ publicKey })`.

---

### Paso 6 — Crear la preferencia de la cuota

Se crea una preferencia de Mercado Pago vinculada a la cuota de la suscripción. El `unit_price` del item es el `installment_amount` del paso anterior.

**Request**

```
POST /api/v1/payments/preference
Authorization: Bearer {token}

{
  "concept": "subscription",
  "description": "Cuota de suscripcion de tier",
  "items": [ { "title": "Cuota mensual tier", "quantity": 1, "unit_price": 1500 } ],
  "installment_id": 501
}
```

**Respuesta (201)**

```json
{
  "preference_id": "1234567890abcdef1234567890",
  "public_key": "APP_USR-..."
}
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `preference_id` | `1234567890abcdef1234567890` (se manda en el pago) |

---

### Paso 7 — Obtener el token de tarjeta (presencia del Card Form)

Hay dos caminos:

**A) Producción: el Bricks genera el token en el front (recomendado).**
El front inicializa el **Payment Brick — Card Form** con la `public_key` y cuando el usuario completa los datos de la tarjeta, el SDK emite un `token` real. **No hay llamada a nuestro backend** para esto: es presencia del SDK de Mercado Pago en la pantalla.

**B) Sandbox / testing: token de tarjeta de prueba vía backend.**

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

**Respuesta (200)**

```json
{ "token": "CARD_TOKEN_REAL_DE_SANDBOX" }
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `card_token` | el `token` de la tarjeta |

> En el entorno de pruebas, la tarjeta Mastercard de sandbox
> `5031 7557 3453 0604` devuelve un pago **aprobado**.

---

### Paso 8 — Procesar el pago de la cuota

Se paga la cuota con el token de tarjeta y la preferencia. El `installment_id` vincula el pago a la cuota de la suscripción.

**Request**

```
POST /api/v1/payments
Authorization: Bearer {token}

{
  "token": "CARD_TOKEN_REAL_DE_SANDBOX",
  "transaction_amount": 1500,
  "payment_method_id": "master",
  "installments": 1,
  "payer_email": "test_user_corredor@example.com",
  "preference_id": "1234567890abcdef1234567890",
  "installment_id": 501
}
```

**Respuesta (200)** — pago aprobado:

```json
{
  "id": 999,
  "payment_id": "200000001234",
  "preference_id": "1234567890abcdef1234567890",
  "concept": "subscription",
  "amount": 1500,
  "status": "approved",
  "status_detail": "accredited",
  "payment_method_id": "master",
  "payer_email": "test_user_corredor@example.com"
}
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `payment.id` | `999` (id local del pago, útil para verificar estado) |
| `payment.payment_id` | `200000001234` (id de Mercado Pago) |

> **El `installment_id` debe ser del usuario autenticado.** El backend valida que
> la cuota exista y que ese id pertenezca a quien hace el pago (el front siempre
> lo cumple, porque lo obtuvo de `subscriptions/current` del propio usuario).

**Errores esperados** (el backend valida la propiedad de la cuota antes de cobrar):

| Código | Status | Significado | Acción del front |
|---|---|---|---|
| `PAYMENT_INSTALLMENT_NOT_FOUND` | 404 | la cuota no existe | recargar / no pagar una cuota inválida |
| `PAYMENT_INSTALLMENT_FORBIDDEN` | 403 | la cuota es de otro usuario | no debería pasar (id vino de su propia suscripción); reportar |
| `Unauthorized` | 401 | no hay usuario autenticado en el contexto | pedir login / renovar session |

> `payer_email` debe ser un **test user** válido de Mercado Pago en sandbox, si no
> la API de MP rechaza el pago.

---

### Paso 9 — Confirmación del pago (el backend activa el tier)

Mercado Pago notifica al backend vía el **webhook**:

```
POST /api/v1/payments/webhook     ← lo invoca MP, NO el front
```

El backend, al recibir la notificación del pago aprobado:
1. **marca la cuota #1 como pagada** (idempotente: si recibe la notificación 2 veces, no duplica),
2. **activa la suscripción** (`status = active`, `paid_installments = 1`),
3. **sincroniza el tier del rol** (`user_roles.tier_id` pasa a `premium`) y
4. **genera la cuota #2** (mensual, con su `due_date` y `blocked_date`) para el próximo mes.

El front **no** invoca el webhook: simplemente puede **hacer polling** del estado del pago y/o consultar la suscripción para ver el resultado. En la colección de Bruno el webhook está como **request opcional** (`10 - Simular webhook MP`), solo para simular la notificación en sandbox.

**Polling opcional del pago (por id local):**

```
GET /api/v1/payments/999
Authorization: Bearer {token}
```

> Cuando `status == "approved"`, la activación ya fue (o está siendo) procesada.

---

### Paso 10 — Verificar el upgrade (cuenta ya activa en el nuevo tier)

Último paso: reconfirmar que quedó **activo** y en **premium**. Devuelve además la **próxima cuota** (la #2) con su vencimiento.

**Request**

```
GET /api/v1/users/42/subscriptions/current?role_id=3
Authorization: Bearer {token}
```

**Respuesta (200)**

```json
{
  "subscription_id": 77,
  "subscription_status": "active",
  "installment_id": 502,
  "installment_number": 2,
  "installment_amount": 1500,
  "next_due_date": "2026-10-03T00:00:00Z",
  "blocked_date": "2026-10-10T00:00:00Z",
  "paid_installments": 1,
  "tier": { "id": 11, "name": "premium", "hierarchy": 2, "payment_required": true },
  "role": { "id": 3, "name": "corredor" },
  "mercadopago": { "public_key": "APP_USR-..." }
}
```

**Qué verificar**

- `subscription_status = "active"` ✅
- `tier.name = "premium"` ✅ (ya tiene el acceso al nuevo tier)
- `paid_installments = 1` ✅
- aparece la **cuota #2** (`installment_id: 502`) con `next_due_date` y `blocked_date` — es la cuota del mes que viene.

> `blocked_date` marca el cutoff de gracia: si la cuota vence y no se paga, se
> vuelve deuda (`DEBT_BLOCKS_OPERATION`) y bloquea nuevas operaciones sobre el rol.

---

## Resumen de endpoints usados (en orden)

| # | Método | Endpoint | Auth | Por qué lo llama el front |
|---|---|---|---|---|
| 1 | `POST` | `/api/v1/auth/login` | no | obtener tokens + `user_id` |
| 2 | `GET` | `/api/v1/auth/permissions?user_id=` | sí | descubrir `role_id` y tier actual |
| 3 | `GET` | `/api/v1/tiers` | sí | descubrir `tier_id` y monto del plan pago |
| 4 | `PUT` | `/api/v1/users/{id}/roles/{role_id}/tier` | sí | ejecutar el cambio de tier (crea sub + cuota) |
| 5 | `GET` | `/api/v1/users/{id}/subscriptions/current?role_id=` | sí | leer cuota a pagar + `public_key` (presencia Bricks) |
| 6 | `POST` | `/api/v1/payments/preference` | sí | crear preferencia de la cuota |
| 7 | `POST` | `/api/v1/payments/test-card-token` | sí | token de tarjeta de prueba (sandbox) |
| 8 | `POST` | `/api/v1/payments` | sí | procesar el pago de la cuota (valida que la cuota exista y sea del usuario) |
| 9 | `POST` | `/api/v1/payments/webhook` | no | lo invoca MP (confirmación/activación) |
| 10 | `GET` | `/api/v1/users/{id}/subscriptions/current?role_id=` | sí | verificar upgrade (active + premium + cuota #2) |

> Paso 7 en producción es **client-side** (Bricks con la `public_key`), no un
> endpoint de backend. El endpoint `/payments/test-card-token` existe solo para
> sandbox/pruebas.

---

## Colección de Bruno

Cada llamada del caso de uso está en `endpoint-collections/CU cambio de tier/`, con
la misma secuencia que el front ejecuta:

| Request Bruno | Qué hace |
|---|---|
| `1 - Login` | obtiene `token` + `user_id` |
| `2 - Descubrir mis roles...` | descubre `role_id` |
| `3 - Descubrir tiers del rol` | descubre `tier_id` + `tier_amount` |
| `4 - Cambiar tier` | ejecuta el cambio |
| `5 - Leer suscripcion actual` | confirma cuota + `public_key` |
| `6 - Crear preferencia` | crea la preferencia |
| `7 - Obtener token de tarjeta` | obtiene `card_token` (sandbox) |
| `8 - Procesar pago` | paga la cuota |
| `9 - Verificar upgrade` | confirma `active` + tier premium (paso 10 del doc) |
| `10 - [Opcional] Simular webhook MP` | simula la notificación (para sandbox) |

> **Variables que se auto-setan** en los scripts `after-response`: `token`,
> `user_id`, `role_id`, `tier_id`, `tier_amount`, `installment_id`,
> `installment_amount`, `public_key`, `preference`, `card_token`, `payment_id` y
> `local_payment_id`. Con eso podés correr la secuencia de corrido.
>
> **`payer_email`**: no la setea ningún request porque el backend no la conoce.
> Seteala a mano en la colección con un **test user** de Mercado Pago (sandbox)
> antes de correr el paso `8 - Procesar pago`.