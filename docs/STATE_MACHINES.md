# State machines del dominio

Referencia única de estados, transiciones, disparadores e invariantes por entidad. La fuente de verdad de los valores de estado son las constantes en `cmd/api/domains/constants/`; este documento es la lectura navegable de cómo se conectan.

Convención del repo: **no se renombran estados existentes** (decisión de `openspec/changes/suscripciones-periodo-y-cancelacion-pendiente`). Los estados nuevos se agregan como constantes nuevas (`canceled`) y quedan fuera de los conjuntos "vigentes" que gobiernan el acceso.

---

## 1. Suscripción de tier — `user_role_tier_subscriptions.status`

Definición: `constants.SubscriptionStatus` (`subscription_status.go`).

| Estado | Significado |
|---|---|
| `first_payment_pending` | Suscripción creada hacia un tier pago; acceso aún no habilitado (espera cuota #1). |
| `active` | Acceso al tier pago habilitado (cuota #1 pagada). |
| `ended` | Ledger cerrado por un cambio de tier (la sub del tier anterior se cierra). |
| `canceled` | Terminal: se abortó la sub con primer pago pendiente. Fuera del conjunto vigente. |

### Transiciones

```
first_payment_pending ──pago de cuota #1──► active
first_payment_pending ──DELETE subscriptions/pending──► canceled
active ──ChangeTier──► ended
```

- `first_payment_pending ─► active`: `MarkPaidConditional` + `Activate` al pagar la cuota #1 (webhook de pago). `user_roles.tier_id` se sincroniza al tier pago en la activación.
- `first_payment_pending ─► canceled`: endpoint `DELETE /api/v1/users/:id/roles/:role_id/subscriptions/pending?tier_id=`; también pasa las cuotas `pending` de esa sub a `canceled` (`CancelPendingBySubscription`), todo en transacción.
- `active ─► ended`: `ChangeTier` (`SetEnded` sobre la sub vigente del tier anterior).

### Disparadores

- `PUT /api/v1/users/:id/roles/:role_id/tier` — cierra la sub vigente y crea la nueva en `first_payment_pending` (target pago) o `active` (target gratis).
- `DELETE /api/v1/users/:id/roles/:role_id/subscriptions/pending?tier_id=` — cancelación (solo en `first_payment_pending`).
- `GET /api/v1/users/:id/subscriptions/:period` — lectura; `current` → `active`, `next` → `first_payment_pending`.
- Asignación de rol (`POST /api/v1/users/:id/roles`) con tier pago — crea la sub en `first_payment_pending` + cuota #1.

### Invariantes

- **Una sola sub vigente por `(user_id, role_id)`**: índice único parcial `uq_sub_ids_user_role_active` (SQL crudo en migración, `postgres.go`) sobre `status IN ('active','first_payment_pending')`. Por esto, elegir la "sub de mayor jerarquía" es no ambiguo: hay a lo sumo una.
- `ended` y `canceled` quedan fuera del conjunto vigente: no cuentan para el índice ni para `FindActiveByUserRole`.
- **`ChangeTier` bloqueado con primer pago impago**: si existe sub `first_payment_pending` para el `(user_id, role_id)`, el `PUT tier` responde `409 SUBSCRIPTION_PENDING_FIRST_PAYMENT`. La vía de salida es cancelarla (DELETE) y reintentar.
- El acceso al tier pago se habilita recién en `active` (cuota #1 pagada).

---

## 2. Cuota — `installments.status`

Definición: `constants.InstallmentStatus` (`installment_status.go`).

| Estado | Significado |
|---|---|
| `pending` | Cuota generada y a pagar. |
| `paid` | Pagada (idempotente ante doble notificación de webhook). |
| `canceled` | Terminal: cancelada junto con la sub en primer pago pendiente. |

### Transiciones

```
pending ──pago (MarkPaidConditional)──► paid
pending ──cancelar suscripción/membresía──► canceled
```

### Disparadores

- `pending ─► paid`: notificación de pago (webhook) → `MarkPaidConditional` (`installment_dao.go`).
- `pending ─► canceled`: `CancelPendingBySubscription` (al cancelar la sub `first_payment_pending`, o al cancelar la membresía de equipo con cuotas pendientes). Las cuotas ya `paid` no se tocan.

### Invariantes

- **Arco exclusivo**: cada cuota referencia exactamente uno de `subscription_id` o `team_id` (CHECK `num_nonnulls(subscription_id, team_id) = 1`).
- **La cuota #1 nunca genera deuda**: las cuotas iniciales de suscripción (`installment_number = 1`) se crean **sin** `due_date`/`blocked_date`; las siguientes (`>= 2`) sí tienen vencimiento.
- `canceled` y `paid` son terminales.

---

## 3. Asignación rol-usuario — `user_roles.status`

| Estado | Significado |
|---|---|
| `active` | Asignación vigente. |
| `inactive` | Asignación desactivada (sin acceso al rol). |

### Transiciones

```
active ──desasignar/desactivar──► inactive
inactive ──reasignar/activar──► active
```

### Disparadores

- `POST /api/v1/users/:id/roles` (AsignRole), `DELETE /api/v1/users/:id/roles/:role_id` (RemoveRole) y actualización de estado de la asignación.

### Invariantes

- El `tier_id` de la asignación refleja el tier con acceso efectivo: puede quedar en el tier gratis mientras una sub pago está `first_payment_pending`, y pasa al tier pagado al activarse la sub (sincronización en la activación).
- Mientras la sub está `first_payment_pending`, la asignación **no** bloquea: `ChangeTier` sigue validando contra la sub, no contra el `tier_id`.

---

## 4. Usuario — `users.status`

Definición: `constants.UserStatus` (`user_status.go`).

| Estado | Significado |
|---|---|
| `active` | Operativo. |
| `inactive` | Desactivado (no loguea). |
| `pause` | En pausa. |
| `blocked` | Bloqueado (fraude/soporte). |
| `suspended` | Suspendido (deuda/reglamento). |

### Transiciones

```
active ⇄ inactive
active ⇄ pause
blocked ──► active        (solo desbloqueo por soporte)
suspended ──► active      (solo resolución)
```

### Disparadores

- Endpoint de actualización de estado del usuario (`PATCH /api/v1/users/:id/status` y variantes del flujo de administración del perfil).

### Invariantes

- `blocked` y `suspended` son terminales salvo intervención explícita de soporte; bloquean operaciones sobre el perfil.

---

## 5. Equipo — `teams.status`

Definición: `constants.TeamStatus` (`team_status.go`).

| Estado | Significado |
|---|---|
| `active` | Equipo operativo. |
| `inactive` | No acepta nuevos miembros. |
| `archived` | Solo lectura. |

### Transiciones

```
active ⇄ inactive
active ──► archived
```

### Disparadores

- Actualización de estado del equipo (cambio de `status` del `Team`, patrón seguido del propio flujo del entrenador).

### Invariantes

- `inactive` no permite agregar miembros ni invitar; `archived` no permite modificar contenido.

---

## 6. Miembro de equipo — `team_users`

Tiene **dos ejes de estado independientes**:

| Eje | Estado | Significado |
|---|---|---|
| `status` | `active` / `inactive` | Asociación del usuario al equipo. |
| `subscription_status` | `first_payment_pending` / `active` | Suscripción a la mensualidad del equipo (`membership_fee`). Null para equipos gratis. |

### Transiciones

```
subscription_status: first_payment_pending ──pago de cuota #1 (con team_id)──► active
al salir del equipo → se baja la fila (no hay estado "ended" de membresía)
```

### Disparadores

- `first_payment_pending`: crear el `team_user` (AddUser o aceptar invitación) con `membership_fee > 0` genera la membresía en `first_payment_pending` + cuota #1 en `installments` con `team_id`.
- `active`: pago de la primera cuota (webhook, patrón `MarkPaidConditional`).
- Salida: RemoveUser / abandonar equipo → `DELETE` de la fila.

### Invariantes

- Acceso pleno (participar/operar en el equipo) recién con `subscription_status = active`.
- La membresía pending no genera deuda (cuota #1 sin vencimiento).
- No hay `ended` de membresía: salirse borra la fila.

---

## 7. Invitación — `invitations.status`

Definición: `constants.InvitationStatus` (`invitation_status.go`).

| Estado | Significado |
|---|---|
| `pending` | Invitación emitida, esperando respuesta. |
| `accepted` | Aceptada (el usuario fue agregado). |
| `rejected` | Rechazada. |

### Transiciones

```
pending ──aceptar──► accepted
pending ──rechazar──► rejected
```

### Disparadores

- Aceptación/rechazo por el invitado (flujo de invitaciones del equipo).

---

## 8. Solicitud de ingreso — `join_requests.status`

Definición: reusa `constants.InvitationStatus` (los mismos 3 valores). Cancelar una solicitud **borra la fila** en vez de agregar un 4to valor (`cancelled`).

| Estado | Significado |
|---|---|
| `pending` | Solicitud emitida, esperando al dueño del equipo. |
| `accepted` | Aceptada por el dueño. |
| `rejected` | Rechazada por el dueño. |

### Transiciones

```
pending ──aceptar (dueño)──► accepted
pending ──rechazar (dueño)──► rejected
pending ──cancelar (autor)──► (fila eliminada)
```

### Disparadores

- `POST /api/v1/teams/:id/join-requests` (crear), `POST /join-requests/:id/accept|reject` (dueño), `DELETE /join-requests/:id` (autor, solo `pending`).

### Invariantes

- El dueño acepta/rechaza; el autor cancela; la cancelación solo procede mientras `pending`.

---

## 9. Conexión vendedor — `seller_connections.status`

Definición: `constants.SellerConnectionStatus` (`seller_connection_status.go`).

| Estado | Significado |
|---|---|
| `authorized` | OAuth mp-connect OK: habilita a cobrar (con split si aplica). |
| `deauthorized` | Conexión revocada/vencida. |

### Transiciones

```
authorized ──revocación/expiración OAuth──► deauthorized
deauthorized ──reconexión mp-connect──► authorized
```

### Invariantes

- Solo `authorized` habilita cobrar con split.
- Sin `authorized`, las suscripciones de contenido pago del rol quedan sin cobrador hasta reconectar.