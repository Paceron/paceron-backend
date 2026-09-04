# Caso de uso: Un miembro paga su participación del equipo

> El **miembro** (corredor) quiere **pagar su cuota mensual** de pertenencia a un
> equipo. El **entrenador** (dueño del equipo) es quien cobra: la plataforma
> arma un pago con **split** (marketplace/plataforma) hacia la cuenta de Mercado
> Pago del entrenador.
>
> A diferencia del caso de uso de cambio de tier (que paga una cuota de
> suscripción `subscription`), acá el concepto es **`team_subscription`** y el
> dinero va al **entrenador**, no a la plataforma.
>
> Este doc es el **paso a paso de endpoints** que el front debe ejecutar para que
> el miembro pague su participación, **incluyendo todas las precondiciones que
> hay que dejar configuradas** (cuenta MP del entrenador, equipo, mensualidad,
> etc.) para poder probar el flujo de punta a punta.
>
> Cada llamada referenciada tiene su request en la colección de Bruno
> `endpoint-collections/CU pago participacion equipo/` (mismo orden/numeración).

---

## Personas y roles (quién es quién)

| Rol | En el sistema | Qué hace |
|---|---|---|
| **Entrenador** | `team.owner_id`, rol global `entrenador`, `team_user.role_in_team = "entrenador"` | dueño del equipo, **es el seller** (cobra la cuota) |
| **Miembro / corredor** | `team_user.role_in_team = "corredor"`, `user_id` de la cuenta | **es quien paga** su cuota |

> La cuota de equipo (`installments`) guarda `user_id = <miembro>` + `team_id`.
> Por eso la validación de propiedad del pago (ver más abajo) comprueba que el
> miembro pague **su propia** cuota.

---

## Precondiciones (lo que hay que configurar para poder probar)

> Estas se dejan preparadas **antes** del flujo de pago. Son las que más suelen
> faltar y las que causan errores tipo `SELLER_NOT_CONNECTED` o "equipo gratis".

### P1. Entrenador con rol global `entrenador`

- El usuario que va a crear el equipo y conectarse a MP debe tener el rol global
  `entrenador` (lo valida `teamService.Create`).
- Se le asigna por el flujo de roles (fuera del alcance de este doc).

### P2. Conexión OAuth del entrenador a Mercado Pago (imprescindible)

El entrenador debe **conectar su cuenta de Mercado Pago** como vendedor. Sin esto,
el pago de equipo falla con **`SELLER_NOT_CONNECTED`** (`"el entrenador debe
conectar su cuenta de Mercado Pago"`).

| Endpoint | Método | Uso |
|---|---|---|
| `GET /api/v1/mercadopago/connect` | GET | genera la URL de OAuth + `state` CSRF → `{auth_url, state}` |
| `GET /api/v1/mercadopago/connect/callback?code=&state=` | GET | intercambio `code` → token, guarda la conexión (redirect de MP) |
| `GET /api/v1/mercadopago/connect/status` | GET | `{connected: bool, account_status: "authorized"\|"deauthorized"}` |

- El **access token** del vendedor se guarda **cifrado** en `seller_connections`.
- La conexión queda `authorized` cuando el exchange fue exitoso; `deauthorized`
  si MP la revoca (webhook).
- **Cómo probarlo rápido:** llamar `GET /mercadopago/connect/status` con el token
  del entrenador y verificar `connected == true`.

### P3. Equipo creado por el entrenador

`POST /api/v1/teams` con el token del entrenador:

```json
{
  "name": "Team Demo",
  "max_members": 10,
  "create_default_group": false
}
```

- El `owner_id` del equipo pasa a ser el entrenador (el "seller").
- `description`, `level`, `requirements` son opcionales; `max_members` es requerido.

### P4. Mensualidad del equipo `membership_fee > 0`  ⚠️ (setup en DB)

La cuota que paga cada miembro sale de **`team.membership_fee`**. El gate que crea
la cuota #1 (`ApplyTeamMembershipGate`) la resuelve así:

- `membership_fee == 0` → membresía **gratis**: queda `subscription_status = active`
  **sin generar cuotas** (no aplica pago).
- `membership_fee > 0` → membidad `first_payment_pending`, `init_amount =
  membership_fee`, y genera la **cuota #1** de `installments`.

> **⚠️ No hay endpoint para setear `membership_fee`** (no está en `POST /teams` ni
> en `PUT /teams/:id`). Para probar tenés que **setearlo en la DB** (seed/SQL):
>
> ```sql
> UPDATE teams SET membership_fee = 5000 WHERE id = <team_id>;
> ```
>
> Si quedó en `0`, el flujo de pago no aplica (el equipo es gratis).

### P5. Comisión de la plataforma (marketplace fee) — opcional, default 5%

El porcentaje que se descuenta para la plataforma se configura como setting global:

| Endpoint | Método | Body |
|---|---|---|
| `GET /api/v1/platform-settings/marketplace-fee` | GET | — |
| `PUT /api/v1/platform-settings/marketplace-fee` | PUT | `{ "marketplace_fee_percent": 5 }` |

- Default **5%**. El fee se calcula sobre `installment.amount` y se guarda en
  `payments.marketplace_fee`.

### P6. Agregar al miembro al equipo (genera la cuota #1)

`POST /api/v1/teams/:id/users` con el token del **entrenador**:

```json
{ "user_id": 42, "role_in_team": "corredor" }
```

- Si el equipo tiene `membership_fee > 0`, esto crea la **cuota #1** del miembro en
  `installments` (status `pending`, `user_id = miembro`, `team_id = equipo`), y la
  membresía queda `first_payment_pending`.

> **Resultado de precondiciones:** miembro agregado con una cuota #1 pendiente por
> pagar. Ya se puede correr el flujo de pago.

---

## Mapa del flujo (vista rápida)

```
                          ┌─▶ descubrir su equipo (member_id) → team_id
 1. Login (miembro) ──────┘
      ▼
 2. Descubrir mi equipo    GET /teams?member_id={user_id}   → team_id
      ▼
 3. Leer mi suscripción    GET /users/{id}/teams/{team_id}/subscription
      │                    → installment_id + installment_amount + checkout (marketplace)
      ▼
 4. Crear preferencia      POST /payments/preference (concept=team_subscription)
      │                    → preference_id (split: token del entrenador + fee)
      ▼
 5. Obtener token tarjeta  (Bricks Card Form en front) / POST /payments/test-card-token
      ▼
 6. Procesar el pago       POST /payments (concept=team_subscription)
      ▼
 7. (MP) confirmar por webhook → la cuota se marca paga, la membresía se activa
      ▼
 8. Verificar membresía    GET /users/{id}/teams/{team_id}/subscription → active
```

---

## Paso a paso detallado

> **Nota de autenticación:** salvo el login y el webhook, todas las llamadas de
> pago llevan el header `Authorization: Bearer {access_token}` del **miembro**.

### Paso 1 — Login del miembro (obtener sesión)

**Request**

```
POST /api/v1/auth/login
Content-Type: application/json

{ "email": "corredor@example.com", "password": "Abcd-12345" }
```

**Respuesta (200)**

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_in": 900,
  "user": { "user_id": 42, "email": "corredor@example.com", "name": "María", "..." : "..." }
}
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `token` | `access_token` |
| `user_id` | `user.user_id` → `42` |

---

### Paso 2 — Descubrir mi equipo (el miembro no conoce el `team_id`)

**Request**

```
GET /api/v1/teams?member_id=42
Authorization: Bearer {token}
```

**Respuesta (200)** — array de equipos donde el usuario es miembro:

```json
[
  {
    "id": 7,
    "name": "Team Demo",
    "owner_id": 5,
    "status": "active",
    "..." : "..."
  }
]
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `team_id` | `7` (el equipo en el que quiere pagar) |

---

### Paso 3 — Leer mi suscripción de equipo (confirmar cuota + checkout)

Consulta la membresía del miembro en el equipo. Devuelve la **cuota a pagar** y
los datos para armar el **checkout Bricks marketplace**.

**Request**

```
GET /api/v1/users/42/teams/7/subscription
Authorization: Bearer {token}
```

> El backend usa el `user_id` del **JWT** del caller (no el `:id` del path), así
> que siempre consulta la membresía del miembro autenticado.

**Respuesta (200)**

```json
{
  "team": { "id": 7, "name": "Team Demo", "membership_fee": 5000 },
  "membership": {
    "subscription_status": "first_payment_pending",
    "init_amount": 5000,
    "paid_installments": 0,
    "start_date": "2026-09-01T00:00:00Z"
  },
  "next_installment": {
    "installment_id": 901,
    "installment_number": 1,
    "installment_amount": 5000,
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

**Qué capturamos**

| Variable | Valor |
|---|---|
| `installment_id` | `901` (la cuota #1 a pagar) |
| `installment_amount` | `5000` |
| `public_key` | `APP_USR-...` (para el Bricks) |
| `mercadopago.concept` | `"team_subscription"` (para el pago) |

> `mercadopago.marketplace = true` le dice al front que arme el **Bricks
> Marketplace** (pago hacia la cuenta del entrenador).

**Errores esperados**

| Código | Status | Significado |
|---|---|---|
| `Not Found` | 404 | equipo o membresía no encontrados |
| `Unauthorized` | 401 | sin token / sin identity en el contexto |

---

### Paso 4 — Crear la preferencia (split hacia el entrenador)

Se crea una preferencia de Mercado Pago **`team_subscription`** vinculada a la
cuota del miembro. El backend resuelve el **split**: toma el token del
entrenador (`resolveTeamSplitConfig`), calcula el `marketplace_fee` y arma el
pago hacia la cuenta del dueño.

**Request**

```
POST /api/v1/payments/preference
Authorization: Bearer {token}

{
  "concept": "team_subscription",
  "description": "Cuota mensual de membresia al equipo",
  "items": [ { "title": "Participacion en el equipo", "quantity": 1, "unit_price": 5000 } ],
  "installment_id": 901
}
```

**Respuesta (201)**

```json
{
  "preference_id": "9876543210abcdef9876543210",
  "public_key": "APP_USR-..."
}
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `preference_id` | `9876543210abcdef9876543210` |

**Errores esperados** — clave acá:

| Código | Status | Significado | Causa |
|---|---|---|---|
| `SELLER_NOT_CONNECTED` | 409 | el entrenador no conectó MP | falta P2 |
| `Not Found` | 404 | la cuota no existe / no es de equipo | installment_id inválido |

> **Gap conocido (a verificar):** el `marketplace_fee` se persiste localmente en
> `payments.marketplace_fee`, pero el request que se envía a Mercado Pago **no
> incorpora el fee/split** en la preferencia ni en el pago. El "split" efectivo
> hoy depende del Bricks Marketplace del frontend (`marketplace=true`). El pago
> se cobra con el **access token del entrenador**. Es un punto a validar/mejorar.

---

### Paso 5 — Obtener el token de tarjeta (el miembro paga)

Igual que en el caso de uso de tier:

**A) Producción:** el Bricks Card Form genera el token en el front con la
`public_key` (no hay llamada a backend).

**B) Sandbox:** `POST /api/v1/payments/test-card-token` con el token del miembro:

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

---

### Paso 6 — Procesar el pago de la cuota de equipo

El **miembro** paga su cuota con su token y la preferencia. El backend **valida
que la cuota sea suya** (que `installment.user_id == usuario autenticado`).

**Request**

```
POST /api/v1/payments
Authorization: Bearer {token}

{
  "token": "CARD_TOKEN_REAL_DE_SANDBOX",
  "transaction_amount": 5000,
  "payment_method_id": "master",
  "installments": 1,
  "payer_email": "maria@example.com",
  "preference_id": "9876543210abcdef9876543210",
  "installment_id": 901,
  "concept": "team_subscription"
}
```

**Respuesta (200)** — pago aprobado:

```json
{
  "id": 1001,
  "payment_id": "200000009999",
  "preference_id": "9876543210abcdef9876543210",
  "concept": "team_subscription",
  "amount": 5000,
  "status": "approved",
  "status_detail": "accredited",
  "payment_method_id": "master",
  "payer_email": "maria@example.com"
}
```

**Qué capturamos**

| Variable | Valor |
|---|---|
| `payment.id` | `1001` (id local) |
| `payment.payment_id` | `200000009999` (id de MP) |

**Errores esperados**

| Código | Status | Significado |
|---|---|---|
| `PAYMENT_INSTALLMENT_NOT_FOUND` | 404 | la cuota no existe |
| `PAYMENT_INSTALLMENT_FORBIDDEN` | 403 | la cuota es de otro usuario (el miembro intenta pagar la cuota ajena) |
| `Unauthorized` | 401 | no hay usuario autenticado |

> `payer_email` debe ser un **test user** de MP válido en sandbox.

---

### Paso 7 — Confirmación del pago (el backend activa la membresía)

Mercado Pago notifica al backend vía el **webhook** (lo invoca MP, NO el front):

```
POST /api/v1/payments/webhook     ← lo invoca MP
```

El backend, al recibir el pago aprobado de una cuota de equipo, en transacción:
1. **marca la cuota como pagada** (idempotente con `MarkPaidConditional`: la
   doble notificación no duplica),
2. **incrementa `paid_installments`** del `team_user`,
3. si era la **cuota #1**, **activa la membresía**
   (`subscription_status = active`),
4. **genera la cuota #2** (mensual, con `due_date` + `blocked_date`).

El front no invoca el webhook: hace **polling** del pago y/o re-consulta la
suscripción.

**Polling opcional del pago (por id local):**

```
GET /api/v1/payments/1001
Authorization: Bearer {token}
```

> Cuando `status == "approved"`, la activación ya fue (o está siendo) procesada.

---

### Paso 8 — Verificar la membresía activa

Re-consulta la suscripción del miembro para confirmar que quedó **activa** y ver
la **próxima cuota**.

**Request**

```
GET /api/v1/users/42/teams/7/subscription
Authorization: Bearer {token}
```

**Respuesta (200)**

```json
{
  "team": { "id": 7, "name": "Team Demo", "membership_fee": 5000 },
  "membership": {
    "subscription_status": "active",
    "init_amount": 5000,
    "paid_installments": 1,
    "start_date": "2026-09-01T00:00:00Z"
  },
  "next_installment": {
    "installment_id": 902,
    "installment_number": 2,
    "installment_amount": 5000,
    "next_due_date": "2026-10-01T00:00:00Z",
    "blocked_date": "2026-10-08T00:00:00Z"
  },
  "has_debt": false,
  "mercadopago": {
    "public_key": "APP_USR-...",
    "concept": "team_subscription",
    "marketplace": true
  }
}
```

**Qué verificar**

- `membership.subscription_status = "active"` ✅
- `paid_installments = 1` ✅
- aparece la **cuota #2** (`installment_id: 902`) con `due_date` y `blocked_date`.

> `blocked_date` es el corte de gracia: si la cuota vence y no se paga, genera
> deuda (`has_debt = true`) y bloquea operaciones sobre el rol/equipo
> (`TEAM_DEBT_BLOCKS_OPERATION`).

---

## Resumen de endpoints usados (en orden)

| # | Método | Endpoint | Auth | Quién llama | Por qué |
|---|---|---|---|---|---|
| (setup) | `GET` | `/api/v1/mercadopago/connect/status` | entrenador | precondición | verificar MP conectado |
| (setup) | `POST` | `/api/v1/teams` | entrenador | precondición | crear el equipo |
| (setup) | `POST` | `/api/v1/teams/:id/users` | entrenador | precondición | agregar miembro (genera cuota #1) |
| 1 | `POST` | `/api/v1/auth/login` | no | miembro | tokens + `user_id` |
| 2 | `GET` | `/api/v1/teams?member_id=` | miembro | miembro | descubrir `team_id` |
| 3 | `GET` | `/api/v1/users/{id}/teams/{team_id}/subscription` | miembro | miembro | leer cuota + checkout |
| 4 | `POST` | `/api/v1/payments/preference` | miembro | miembro | crear preferencia (split) |
| 5 | `POST` | `/api/v1/payments/test-card-token` | miembro | miembro | token de tarjeta (sandbox) |
| 6 | `POST` | `/api/v1/payments` | miembro | miembro | procesar el pago |
| 7 | `POST` | `/api/v1/payments/webhook` | no | MP | confirmar + activar membresía |
| 8 | `GET` | `/api/v1/users/{id}/teams/{team_id}/subscription` | miembro | miembro | verificar activa |

---

## Colección de Bruno

Cada llamada del caso de uso está en `endpoint-collections/CU pago participacion equipo/`:

- `1 - Login miembro`
- `2 - Descubrir mi equipo`
- `3 - Leer mi suscripcion equipo`
- `4 - Crear preferencia`
- `5 - Obtener token tarjeta`
- `6 - Procesar pago`
- `7 - [Opcional] Simular webhook MP`
- `8 - Verificar membresia activa`

> Variables que se auto-setan en los scripts `after-response`: `token`,
> `user_id`, `team_id`, `installment_id`, `installment_amount`, `public_key`,
> `preference`, `card_token`, `payment_id`.
>
> **`payer_email`** no la setea ningún request: seteala a mano con un **test user**
> del miembro antes de correr el paso `6 - Procesar pago`.