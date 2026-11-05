# Tareas: Cobros pendientes, vencimientos y comisión en todos los cobros

## 1. Change de OpenSpec

- [x] 1.1 Escribir proposal, design, spec y tasks. Validar con `openspec validate --strict`.

## 2. Datos

- [x] 2.1 Completar en testing los 27 cobros `team_subscription` sin `marketplace_fee` con 5% del monto (script manual, decisión del equipo; ver design.md Context).

## 3. Comisión en el código de pagos

- [x] 3.1 `CreatePreference` y `ProcessPayment`: guardar `marketplace_fee` en todo `team_subscription`, también en 0.
- [x] 3.2 `ProcessPayment`: resolver el split por `concept`, así un `team_subscription` sin `installment_id` devuelve `400`.
- [x] 3.3 Tests de los dos casos.

## 4. Neto = bruto − comisión

- [x] 4.1 DAO del historial: neto `amount − marketplace_fee` (5% si falta), y `fee_amount` y `fee_percent` en cada cobro.
- [x] 4.2 Ajustar los tests del DAO y del service que esperaban el neto de Mercado Pago.

## 5. Obligaciones calculadas

- [x] 5.1 Funciones puras en `services/payment_obligations.go`: cuotas esperadas, próxima pendiente y vencido en hora argentina.
- [x] 5.2 Tests por tabla: alta a fin de mes, sin cuotas pagas, al día, varias vencidas, vence hoy, monto 0.

## 6. DAO de obligaciones

- [x] 6.1 Membresías con cuota de los equipos vivos de un entrenador, con corredor y equipo.
- [x] 6.2 Membresías con cuota y suscripciones de tier de un usuario, con equipo, entrenador y tier.
- [x] 6.3 Tests contra Postgres y ramas de error con `FailingDB`.

## 7. Resumen y endpoint nuevo

- [x] 7.1 `GetReceivedSummary`: pendiente por mes, `due_this_month` y `overdue` (vencidas de cualquier mes).
- [x] 7.2 `GetUpcoming` y `GET /api/v1/payments/upcoming`, con controller, ruta y godoc.
- [x] 7.3 Tests del service y del controller (200, 401, 500).
- [x] 7.4 Los `500` del historial de pagos devolvían el texto del error, que envuelve el de la DB. Pasan a responder "error interno", como el resto de los controllers nuevos.

## 8. Verificación y documentación

- [x] 8.1 `EXPLAIN ANALYZE` de las consultas nuevas contra testing: la del entrenador planifica con seq scan y tarda 0,12 ms (`team_users` 49 filas, `teams` 19). Se agrega igual el índice en `teams.owner_id` (design D8).
- [x] 8.2 Regenerar swagger. Actualizar README y `docs/DEUDA_TECNICA_Y_PENDIENTES.md`.
- [x] 8.3 `go vet ./...`, `go test ./...` y validar el change.

## 9. Salida del equipo con deuda

- [x] 9.1 `membershipDebt`/`membershipOverdue` en `payment_obligations.go`, con tests.
- [x] 9.2 `RemoveUser`: el corredor no puede irse con cuotas que vencen hoy o antes (`ErrTeamLeaveWithDebt`, `409`); el entrenador sí lo puede sacar (design D7).
- [x] 9.3 `has_debt` del estado de cuenta con el mismo cálculo.
- [x] 9.4 Tests del service y del controller.
- [x] 9.5 Datos de testing: "Los cracks" a $12.000 y la membresía de findmejuan desde el 22/08 con la cuota #1 (script manual, decisión del equipo).
- [x] 9.6 Montos de las cuotas pendientes y rechazadas del resumen (`pending_amount`, `rejected_amount` y sus netos), con test y swagger.
