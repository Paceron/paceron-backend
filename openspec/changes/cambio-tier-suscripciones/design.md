# Design: Cambio de tier con suscripciones

## Context

Hoy `POST /api/v1/users/:id/roles` (`services/user_role_service.go`) asigna rol + tier sin registrar cómo se paga, y no hay forma de validar deuda ni de cambiar de tier con reglas de negocio. Queremos cobrar los tiers pagos (mensualidad por adelantado), llevar un ledger de suscripciones y cuotas, y permitir cambiar de tier (mismo rol, hacia arriba o abajo) con validación de deuda.

Este change cubre la suscripción **individual** (usuario → Paceron). La suscripción de corredor a entrenador (equipos + split payment) es el change `suscripcion-teams-split`, pero `installments` se diseña **compartida** desde ahora para no migrar la tabla.

## Goal

- Cobrar/controlar tiers pagos con un ledger de suscripciones (`user_role_tier_subscriptions`) y cuotas (`installments`).
- Activar el acceso a un tier pago recién cuando se paga la primera cuota.
- Permitir cambiar de tier dentro del mismo rol (arriba/abajo) bloqueando si hay deuda.
- Exponer `GET` de próxima cuota a pagar para armar el checkout Bricks desde el frontend.

## Non-Goals

- Procesar el pago dentro del endpoint de próxima cuota (el pago sigue pasando por `POST /api/v1/payments/preference` + `POST /api/v1/payments` + webhook).
- Programar cobros automáticos/recibos recurrentes (MP no se encarga; el backend genera la cuota siguiente al confirmarse la anterior).
- Suscripción de equipos/split payment (change `suscripcion-teams-split`); acá solo se define la columna `team_id` en `installments`.

## Decisions

### D1. Modelo de datos: subscriptions como ledger + cuotas en `installments` (arco exclusivo)

`user_role_tier_subscriptions`:

| columna | tipo | notas |
|---|---|---|
| `id` | PK | |
| `user_id` | int NOT NULL | FK → users |
| `role_id` | int NOT NULL | FK → roles |
| `tier_id` | int NOT NULL | FK → tiers |
| `status` | string NOT NULL | `first_payment_pending` / `active` / `ended` |
| `init_amount` | numeric NOT NULL | monto de la cuota mensual (= `tier_amount`) |
| `paid_installments` | int NOT NULL default 0 | contador denormalizado, se actualiza en la misma transacción del webhook |
| `start_date` | timestamp NOT NULL | |
| `ended_date` | timestamp NULL | al cerrar por cambio de tier |
| `created_at` / `updated_at` | | |

Constraints:
- **Índice único parcial** `(user_id, role_id)` solo sobre registros vigentes (`status IN ('active','first_payment_pending')`) — garantiza "una sola sub vigente por usuario y rol" y permite el historial/ledger. GORM no expresa índices parciales por tag: se crea con SQL crudo en la migración (ver D10).
- Terna `(user_id, role_id, tier_id)` NOT NULL en todo registro.

`installments`:

| columna | tipo | notas |
|---|---|---|
| `id` | PK | |
| `subscription_id` | int NULL | FK → `user_role_tier_subscriptions.id` (cuotas individuales) |
| `team_id` | int NULL | FK → `teams.id` (suscripción a equipo, change 2) |
| `user_id` | int NOT NULL | quién debe |
| `installment_number` | int NOT NULL | arranca en 1 |
| `status` | string NOT NULL | `pending` / `paid` |
| `internal_payment_id` | int NULL | FK → `payments.id` |
| `external_payment_id` | string NULL | `payment_id` de Mercado Pago |
| `amount` | numeric NOT NULL | |
| `due_date` | timestamp NULL | nulo en cuota #1 (no venció nada todavía) |
| `blocked_date` | timestamp NULL | nulo en cuota #1; cutoff de gracia para las siguientes |
| `created_at` / `updated_at` | | |

- **CHECK de arco exclusivo**: exactamente uno de `subscription_id` o `team_id` seteado (`num_nonnulls(subscription_id, team_id) = 1`). Se crea con SQL crudo (D10); el change 1 solo usa `subscription_id`, `team_id` queda listo para el change 2.

### D2. AssignRole con tier pago: acceso mínimo hasta el primer pago

En `POST /api/v1/users/:id/roles`:
- Si `tier.PaymentRequired == false` → comportamiento actual + `tier_id` = tier pasado.
- Si `tier.PaymentRequired == true` → en una **transacción GORM**: (1) crea `user_roles` con `tier_id` = **tier de menor jerarquía (base)** de ese rol — el usuario arranca con el acceso gratis y deja el tier pago para cuando pague; (2) crea la suscripción `first_payment_pending` con `init_amount = tier_amount`; (3) crea la cuota #1 `pending`, `amount = tier_amount`, sin `due_date`/`blocked_date`.

Cobro por adelantado: el acceso al tier pago (y por ende lo que se debe mes a mes) arranca recién cuando se paga la cuota #1.

### D3. Sincronización del tier vigente (Opción A, acordada)

`user_roles.tier_id` es la fuente de verdad del acceso y se sincroniza en el webhook del primer pago:
- Asignación paga: `user_roles.tier_id` = tier base → al confirmarse la cuota #1 → `user_roles.tier_id` = tier pago.
- Cambio de tier pago: `user_roles.tier_id` conserva el tier anterior (el usuario conserva su acceso actual) → al pagar la primera cuota del nuevo → se actualiza al target.
- Cambio de tier gratis: actualización inmediata (no hay cuota).

Alternativa descartada (B): validar la suscripción vigente en cada consulta de acceso. Más código y acoplaba permisos al estado de pagos; con (A) el gate es natural por el valor de `tier_id`.

### D4. ChangeTier (`PUT /api/v1/users/:id/roles/:role_id/tier`)

Validaciones, en orden:
1. Existe `user_roles` para `(user_id, role_id)` (asignación previa requerida).
2. Target tier tiene `role_id` igual al de la asignación (nunca `role_name`).
3. **Sin deuda** (D5).
4. **No hay primer pago impago**: si la sub vigente está `first_payment_pending` con la cuota #1 sin pagar, se rechaza — evita encadenar cambios de tier pago sin pagar nunca (anti-abuso). (Si la cuota #1 ya quedó pagada y pasó a `active`, es un cambio normal.)

Si pasa:
- Cierra la sub vigente: `status = ended`, `ended_date = now`.
- Crea la nueva sub:
  - Target pago → `first_payment_pending` + cuota #1 (`pending`, sin vencer). `user_roles.tier_id` se mantiene en el tier vigente hasta el pago (D3).
  - Target gratis → `active`, sin cuota; `user_roles.tier_id` = target de inmediato.

Todo en una transacción GORM.

### D5. Deuda bloqueante

Una cuota genera **deuda** si es `pending` y su `blocked_date` es anterior a hoy (equivale a `due_date` vencida + período de gracia vencido). Reglas:
- La cuota #1 **nunca** genera deuda (no tiene `due_date`).
- Solo cuentan las cuotas de la sub vigente (`active` o `first_payment_pending`). Las de subs `ended` no.
- La deuda bloquea el cambio de tier (D4). No existiendo aún otros flujos sensibles a deuda, queda naturalmente extensible (ej. borrar cuenta, dejar equipo).

### D6. Ciclo de cuotas mensuales (generación perezosa)

Al confirmarse el pago de la cuota `N` (webhook), se genera la cuota `N+1` en la misma transacción:

- `installment_number = N + 1`
- `status = pending`
- `amount = init_amount` de la suscripción (cuota mensual fija)
- `due_date` = `start_date + 1 mes` si `N == 1`; si no, `due_date` de la cuota `N + 1 mes` (`time.AddDate(0, 1, 0)`)
- `blocked_date = due_date + 7 días` (gracia)
- Sin pago, no se generan más cuotas: el "account status" muestra la cuota pendiente actual (y deuda si se pasó el `blocked_date`).

### D7. Webhook idempotente y transaccional

Flujo al confirmarse un pago `approved` (extiende `payment_service.go`):
1. Resolver el `payments` local (lógica existente por ID / external reference).
2. Si `payments.installment_id` está seteado (pago de cuota), en una transacción:
   - **Update condicional** `installments SET status='paid' WHERE id=? AND status='pending'` — de este marcador depende todo lo siguiente (idempotencia).
   - Si el update afectó 1 fila: incrementar `paid_installments`, y si fue cuota #1 → sub `active` + `user_roles.tier_id` = tier de la sub (D3) + generar cuota #2 (D6). Si fue cuota `N>1` → generar cuota `N+1`.
   - Setear `internal_payment_id` / `external_payment_id` en la cuota.
3. El update condicional garantiza que la doble notificación del webhook no tenga efectos.

### D8. Vínculo pago ↔ cuota

- `payments` gana columna nullable `installment_id` FK → `installments.id` (relación de pago a cuota). La `external_reference` existente se mantiene (id del `payments` local).
- `CreatePreferenceRequest` gana `installment_id` **opcional**: al crearla, el backend registra el `payments` con `installment_id` + `concept = "subscription"`. Así el webhook mapea pago → cuota sin depender de parsing de `external_reference`.
- `ProcessPayment` directo (sin preference) también puede recibir `installment_id`. En iteración 1 el flujo Bricks usa preference.

### D9. Endpoints

- `PUT /api/v1/users/:id/roles/:role_id/tier` — body `{ "tier_id": int }`. Devuelve la nueva sub (o error de validación/deuda).
- `GET /api/v1/users/:id/subscriptions/current?role_id=X` — próxima cuota a pagar de la sub vigente. Respuesta:
  ```json
  {
    "subscription_id": 3,
    "subscription_status": "first_payment_pending",
    "installment_id": 7,
    "installment_number": 1,
    "installment_amount": 1500.0,
    "next_due_date": null,
    "blocked_date": null,
    "paid_installments": 0,
    "tier": { "id": 2, "name": "premium", "hierarchy": 3, "payment_required": true },
    "role": { "id": 1, "name": "corredor" },
    "mercadopago": { "public_key": "APP_USR-..." }
  }
  ```
  Si el rol es gratis (`payment_required=false`, sin cuota), devuelve el estado del rol/tier y `pending_installment: null`.
- `POST /api/v1/users/:id/roles` — modifica su comportamiento (D2, D3).
- `POST /api/v1/payments/preference` — acepta `installment_id` opcional (D8).

### D10. Migración (postgres.go)

GORM tag model + SQL crudo después de `AutoMigrate`:
- `CREATE UNIQUE INDEX IF NOT EXISTS uq_sub_ids_user_role_active ON user_role_tier_subscriptions (user_id, role_id) WHERE status IN ('active','first_payment_pending');`
- CHECK de arco exclusivo en `installments` vía bloque `DO` (evita error si ya existe): `num_nonnulls(subscription_id, team_id) = 1`.
- `payments.installment_id` como FK nullable (columna nueva sobre tabla existente → aditiva, no rompe).

### D11. Errores de dominio

DTOs de respuesta de error por código (usar ya existentes en `domains/apierror`): custom codes para `TIER_NOT_FOUND`, `TIER_ROLE_MISMATCH`, `SUBSCRIPTION_PENDING_FIRST_PAYMENT`, `DEBT_BLOCKS_OPERATION`.

## Risks / Trade-offs

- **Índice único parcial + GORM**: GORM puede migrar la tabla sin el índice parcial (lo creamos post-AutoMigrate con SQL crudo). En tests de DAO con Postgres real (`testutils.SetupTestDB`) hay que correr la misma migración.
- **`user_roles.tier_id` como sincronización**: si el webhook falla entre cuota #1 y el update de `tier_id`, la transacción D7 los protege (mismo commit). Manualmente podría quedar desincronizado solo si se toca la DB.
- **Abuso por encadenamiento de cambios**: mitigado con la regla D4.4 (no cambiar con primer pago impago).
- **Cuota fija**: `init_amount` no cambia si el admin edita `tier_amount` después; el cambio de monto aplica a cambios/suscripciones nuevas. Se documenta para no abrir una migración de deuda intermedia.

## Follow-up

- Change `suscripcion-teams-split`: usa `installments.team_id`, cobra al entrenador con split (`marketplace_fee`), estado de suscripción en `team_users`, endpoints `GET /api/v1/users/:id/teams/:team_id/subscription`.
- Recurrencia real (cobros automáticos, tarjeta guardada) si el modelo de cuotas perezosas no alcanza.