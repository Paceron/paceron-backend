## ADDED Requirements

### Requirement: Comisión en todo cobro de equipo

El sistema MUST guardar `marketplace_fee` en todo pago con `concept = team_subscription`, también cuando la comisión calculada vale 0. El sistema MUST rechazar con `400` un pago `team_subscription` que no traiga `installment_id`.

#### Scenario: Comisión que redondea a cero

- **WHEN** el porcentaje vigente hace que la comisión de una cuota redondee a 0
- **THEN** el pago se guarda con `marketplace_fee = 0`, no `NULL`

#### Scenario: Pago de equipo sin cuota

- **WHEN** llega un `POST /payments` con `concept = team_subscription` y sin `installment_id`
- **THEN** el backend responde `400`
- **AND** no se contacta a Mercado Pago

### Requirement: Neto de un cobro

El sistema MUST informar el neto de todo cobro aprobado como `amount − marketplace_fee`. Si el cobro no tuviera comisión guardada, MUST usar 5% del monto. Cada cobro MUST exponer `fee_amount` y `fee_percent`.

#### Scenario: Cobro aprobado con comisión

- **WHEN** un cobro aprobado de 20.000 tiene `marketplace_fee = 1.000`
- **THEN** `net_amount = 19.000`, `fee_amount = 1.000` y `fee_percent = 5`

#### Scenario: Resumen sin neto parcial

- **WHEN** el entrenador pide el resumen de cobros
- **THEN** cada mes y cada equipo trae `net_known_count` igual a `approved_count`

### Requirement: Obligaciones calculadas

El sistema MUST derivar las cuotas esperadas de cada membresía y suscripción con monto mayor a 0. La #1 vence en el alta y cada cuota siguiente un mes después de la anterior, igual que el motor de cuotas. Las primeras `paid_installments` están pagadas. Una obligación sin pagar está vencida si su vencimiento es anterior a hoy, en hora argentina.

#### Scenario: Corredor que dejó de pagar

- **WHEN** una membresía con alta 2026-09-06 tiene 1 cuota pagada y hoy es 2026-11-02
- **THEN** la cuota 2 (2026-10-06) está pendiente y vencida
- **AND** la cuota 3 (2026-11-06) está pendiente y a vencer

#### Scenario: Membresía sin cuota

- **WHEN** una membresía tiene `init_amount = 0`
- **THEN** no genera obligaciones

#### Scenario: Corredor que dejó el equipo

- **WHEN** la membresía tiene `deleted_at`
- **THEN** no genera obligaciones

### Requirement: Cobros pendientes en el resumen

El resumen de cobros MUST sumar en cada mes de la ventana las obligaciones sin pagar que vencen en ese mes (`pending_amount`, `pending_net_amount`, `pending_count`). También MUST devolver `due_this_month` (vence desde hoy hasta fin de mes) y `overdue` (todo lo que ya venció sin pagar, de cualquier mes), con sus ítems.

#### Scenario: Pendiente por mes

- **WHEN** el entrenador tiene una obligación sin pagar que venció en octubre
- **THEN** el mes de octubre trae ese monto en `pending_amount`

#### Scenario: Vence hoy

- **WHEN** una obligación vence hoy
- **THEN** cuenta en `due_this_month` y no en `overdue`

#### Scenario: Vencida de un mes anterior

- **WHEN** una obligación venció el 22/10 y hoy es 2026-11-02
- **THEN** cuenta en `overdue` aunque sea de octubre

#### Scenario: Cobros de tier

- **WHEN** existen pagos de tier
- **THEN** no aparecen en ningún bloque del resumen del entrenador

### Requirement: Próximos vencimientos del usuario

El sistema MUST devolver en `GET /payments/upcoming` la próxima obligación sin pagar de cada membresía con cuota y de cada suscripción de tier del usuario autenticado, ordenadas por vencimiento, con `overdue` y `overdue_count`.

#### Scenario: Cuota de equipo y tier

- **WHEN** el usuario paga un equipo y tiene un tier pago
- **THEN** la respuesta trae un ítem `trainer_payment` y uno `subscription`

#### Scenario: Varias cuotas vencidas

- **WHEN** una membresía con alta 2026-08-10 tiene 1 cuota pagada y hoy es 2026-11-02
- **THEN** el ítem es la cuota 2 (2026-09-10) con `overdue = true`
- **AND** `overdue_count = 2`, porque también venció la 3 (2026-10-10) y la 4 (2026-11-10) todavía no

#### Scenario: Sin cuotas

- **WHEN** el usuario no tiene membresías con cuota ni tier pago
- **THEN** la respuesta es `200` con `items` vacío

### Requirement: Salir del equipo con cuotas pendientes

El sistema MUST rechazar con `409` que un corredor deje un equipo si su membresía tiene cuotas sin pagar que vencen hoy o antes, incluida la #1. La deuda MUST calcularse desde la membresía, sin depender de que la cuota exista como fila ni de la cuota actual del equipo. El entrenador MAY sacar a un corredor con deuda.

#### Scenario: Corredor con cuotas vencidas

- **WHEN** un corredor que se unió el 2026-08-22 a $12.000 sin pagar ninguna cuota se quiere ir el 2026-11-02
- **THEN** la respuesta es `409` y dice que tiene 3 cuotas sin pagar
- **AND** la membresía sigue activa

#### Scenario: Cuota #1 sin pagar

- **WHEN** un corredor se une hoy a un equipo pago y se quiere ir sin pagar la #1
- **THEN** la respuesta es `409`

#### Scenario: Corredor al día

- **WHEN** el corredor pagó todas las cuotas que vencen hasta hoy
- **THEN** puede dejar el equipo

#### Scenario: El entrenador saca a un corredor que debe

- **WHEN** el entrenador saca del equipo a un corredor con cuotas vencidas
- **THEN** la baja se hace y no se generan más cuotas

#### Scenario: Equipo que pasó a ser gratis

- **WHEN** el corredor se unió pagando, el equipo después bajó la cuota a 0 y tiene cuotas vencidas
- **THEN** no puede irse y `has_debt` es `true`
