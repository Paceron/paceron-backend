# Diseño: Cobros pendientes, vencimientos y comisión en todos los cobros

## Context

Relevado en develop el 2026-11-02:

- **Cómo se calcula la comisión hoy.** `resolveTeamSplitConfig` calcula `cuota × marketplace_fee_percent / 100`, redondeado a 2 decimales, con 5% si el ajuste falta o no es válido. El resultado se guarda en `payments.marketplace_fee` como monto, solo si es mayor que 0. Una comisión que redondea a 0 queda en `NULL`.
- **De dónde salían los cobros sin comisión.**
  - 18 filas (ids 169 a 186) se cargaron por SQL, sin comisión.
  - 9 filas se crearon el 2026-10-06 con el ajuste en 0.00001%. La comisión redondeó a 0 y quedó `NULL`.
  - Los 77 cobros restantes estaban en 5.00% exacto. Un script en testing completó los 27 con 5% del monto, y hoy los 104 están en 5%.
- **El neto.** El historial lo lee de `raw_response.transaction_details.net_received_amount`, que solo trae el webhook y que además descuenta el costo de MP (20.000 → 19.148,35).
- **Pago de equipo sin cuota.** `ProcessPayment` resuelve el split solo si `concept = team_subscription` **y** viene `installment_id`. Sin la cuota, el pago sale sin split y sin comisión, con el token de Paceron.
- **Las cuotas se crean de a una.**
  - La #1 nace al crear la membresía o la suscripción, sin vencimiento.
  - La N+1 la crea el webhook cuando se aprueba la N. Su vencimiento es el alta + N meses, y el bloqueo, el vencimiento + 7 días (`installment_engine.go`).
  - En los 8 corredores con cuota de develop, `paid_installments` coincide con las cuotas pagas y la próxima vence en alta + `paid_installments` meses.
- **Los cobros de tier no le corresponden al entrenador.** `ListReceived` ya filtra `concept = team_subscription`: los pagos de tier van a Paceron y cada usuario los ve en su propio historial.

## Decisions

### D1. Todo cobro de equipo guarda su comisión

`CreatePreference` y `ProcessPayment` guardan `marketplace_fee` para todo pago `team_subscription`, también cuando vale 0. `NULL` queda reservado para pagos sin split (tier).

`ProcessPayment` resuelve el split por el `concept` y no por la presencia de la cuota. Así, un `team_subscription` sin `installment_id` recibe el `400` que `resolveTeamSplitConfig` ya devuelve, en vez de cobrarse con el token de Paceron.

### D2. Neto = bruto − comisión

Decisión del equipo (2026-11-02). Reemplaza la D5 de `historial-pagos-cobros-entrenador`, que mostraba solo el neto informado por Mercado Pago.

- **Cálculo:** el neto de un cobro aprobado es `amount − marketplace_fee`. Si un dato viejo no tuviera comisión, se toma 5% del monto (el mismo valor por defecto de `resolveTeamSplitConfig`). No queda ningún cobro sin neto.
- **Lo que se expone:** cada cobro suma `fee_amount` y `fee_percent`.
- **Resumen:** `net_known_count` pasa a ser igual a `approved_count`. Se mantiene en el contrato para no romper el frontend.

### D3. Las obligaciones se calculan, no se guardan

Una obligación es una cuota esperada. Para una membresía o suscripción con monto `m` mayor a 0, alta `a` y `p` cuotas pagadas:

- La #1 vence en el alta. El backend no la cuenta como deuda para bloquear, pero para el entrenador es plata pendiente desde ese día.
- La #2 vence un mes después del alta, y cada una de las siguientes un mes después de la anterior. Es la misma cadena que `BuildNextInstallment`. No equivale a `a + (k − 1)` meses: `AddDate` normaliza las fechas de fin de mes (31/1 + 1 mes = 3/3), y la cadena tiene que dar igual que la cuota que crea el webhook.
- Las cuotas `1..p` están pagadas. De `p + 1` en adelante, pendientes.
- Su monto es `init_amount`, el mismo que usa `BuildNextInstallment`.
- Entran las membresías vivas (`team_users.deleted_at IS NULL`, `role_in_team = 'corredor'`, `init_amount > 0`) de los equipos vivos del entrenador (`teams.owner_id`).
- Para tier entran las suscripciones `active` y `first_payment_pending` con `init_amount > 0`.

No se agrega ninguna tabla. Generar las cuotas de meses no pagados cambiaría el motor de cuotas y su webhook, y el cálculo da lo mismo sin escribir nada. Vive en funciones puras, testeadas por tabla.

### D4. Vencido es pasar el vencimiento

Decisión del equipo. Una obligación pendiente está vencida si su vencimiento ya pasó, comparando fechas en hora argentina. Los 7 días de gracia hasta `blocked_date` no cambian nada acá: siguen definiendo solo el bloqueo del corredor.

### D5. El resumen de cobros suma lo pendiente

`GET /payments/received/summary` agrega, sin quitar nada:

- en cada mes de `monthly`: `pending_amount`, `pending_net_amount` y `pending_count`. Son las obligaciones sin pagar que vencen en ese mes. La ventana es la misma del gráfico, así que se navega igual con `until`.
- `due_this_month`: lo que vence desde hoy hasta fin del mes actual.
- `overdue`: todo lo que ya venció sin pagar, de este mes o de meses anteriores. Una cuota de octubre que no se pagó se sigue debiendo en noviembre; con solo las del mes, un corredor que dejó de pagar hace tres meses no aparecería en ningún lado (caso real de testing, 2026-11-02).

Los dos bloques traen `count`, `amount`, `net_amount` e `items` (corredor, equipo, número de cuota, monto y vencimiento) y no dependen de la ventana. El neto pendiente usa el porcentaje de comisión vigente, porque ese pago todavía no tiene comisión guardada.

### D6. Próximos vencimientos del usuario

`GET /api/v1/payments/upcoming` devuelve, para el usuario autenticado y en cualquier rol, la próxima obligación pendiente (`p + 1`):

- de cada membresía con cuota (`type = trainer_payment`, con equipo y entrenador);
- de cada suscripción de tier (`type = subscription`, con tier y rol).

Cada ítem trae `installment_number`, `amount`, `due_date` y `overdue`. Si hay más de una cuota vencida, `overdue_count` dice cuántas. Se ordenan por vencimiento.

### D7. No se puede dejar el equipo con cuotas pendientes

El corredor no puede irse de un equipo (`DELETE /teams/:id/users/:user_id` sobre sí mismo) si tiene cuotas que vencen hoy o antes, incluida la #1, que vence en el alta. Responde `409` con "no podés dejar el equipo con cuotas pendientes: tenés N cuotas sin pagar".

La deuda se calcula desde la membresía con las mismas reglas de D3, en el momento: no depende de que la cuota exista como fila ni de la cuota que tenga hoy el equipo. El chequeo anterior solo corría si el equipo cobraba en ese momento, solo miraba cuotas ya creadas y nunca contaba la #1, así que alguien podía unirse, no pagar nunca e irse. Las cuotas pending que ya existen como fila se siguen mirando, por si el contador de pagadas quedó desfasado.

El entrenador sí puede sacar a un corredor que debe (decisión del equipo, 2026-11-02): sacarlo perdona la deuda y corta las cuotas, porque la membresía deja de existir. Salir de un grupo no toca la membresía del equipo, así que las cuotas siguen.

`has_debt` del estado de cuenta (`GET /users/:id/teams/:team_id/subscription`) usa el mismo cálculo, con cuotas ya vencidas (vencer hoy no es estar vencida), y una membresía que se dio de alta pagando sigue mostrando su deuda aunque el equipo haya pasado a ser gratis.

### D8. Índice en `teams.owner_id`

Las consultas leen `team_users` por `team_id` y `user_id`, y `user_role_tier_subscriptions` por `user_id`: esas columnas ya tienen índice. La lista de membresías de un entrenador se filtra por `teams.owner_id`, que no lo tenía. Hoy `teams` tiene 19 filas y el plan tarda 0,12 ms con seq scan, pero el resumen de cobros la corre en cada request y crece con la cantidad de equipos. Se agrega el índice con el tag de GORM (`AutoMigrate` lo crea al arrancar); también lo aprovechan las demás consultas de "mis equipos".

## Contrato

```
GET /api/v1/payments/received/summary        (cambios)
  monthly[]:            + pending_amount, pending_net_amount, pending_count
  + due_this_month:     { count, amount, net_amount, items[] }
  + overdue:            { count, amount, net_amount, items[] }
    items[]: { team: {id, name}, payer: {id, name, surname}, installment_number, amount, due_date }
  net_amount y net_known_count: ahora siempre informados para los aprobados (D2)

GET /api/v1/payments/received                (cambios)
  payments[]: net_amount siempre en aprobados; + fee_amount, fee_percent

GET /api/v1/payments/upcoming                (nuevo)
→ 200 { items: [{ type: "trainer_payment" | "subscription",
                  team?: {id, name}, trainer?: {id, name, surname},
                  tier?: {id, name, role_name},
                  installment_number, amount, currency_id, due_date, overdue, overdue_count }] }
→ 401 · 500

DELETE /api/v1/teams/:id/users/:user_id     (cambios)
→ 409 si el corredor se quiere ir con cuotas que vencen hoy o antes (D7)
```

Las fechas van en RFC 3339 UTC, como el resto del historial.

## Risks / Trade-offs

- **El cálculo supone que el ciclo no se rompe.** Si alguna vez se cambia el monto de una membresía sin actualizar `init_amount`, o se crea una cuota fuera del motor, la obligación calculada no va a coincidir. Hoy las 8 membresías coinciden.
- **La #1 cuenta como pendiente.** Un corredor que se une y no paga aparece debiendo desde el alta y no puede irse hasta pagarla (D7). Si se arrepiente el mismo día, el entrenador lo puede sacar.
- **El neto pendiente es estimado.** Usa el porcentaje vigente: si se cambia antes de que el corredor pague, el neto real va a ser otro.
