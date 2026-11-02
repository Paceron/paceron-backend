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

El resumen de cobros MUST sumar en cada mes de la ventana las obligaciones sin pagar que vencen en ese mes (`pending_amount`, `pending_net_amount`, `pending_count`). También MUST devolver `due_this_month` (vence desde hoy hasta fin de mes) y `overdue_this_month` (venció desde el 1° del mes hasta ayer), con sus ítems.

#### Scenario: Pendiente por mes

- **WHEN** el entrenador tiene una obligación sin pagar que venció en octubre
- **THEN** el mes de octubre trae ese monto en `pending_amount`

#### Scenario: Vence hoy

- **WHEN** una obligación vence hoy
- **THEN** cuenta en `due_this_month` y no en `overdue_this_month`

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
