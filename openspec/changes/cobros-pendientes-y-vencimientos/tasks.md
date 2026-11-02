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

- [ ] 6.1 Membresías con cuota de los equipos vivos de un entrenador, con corredor y equipo.
- [ ] 6.2 Membresías con cuota y suscripciones de tier de un usuario, con equipo, entrenador y tier.
- [ ] 6.3 Tests contra Postgres y ramas de error con `FailingDB`.

## 7. Resumen y endpoint nuevo

- [ ] 7.1 `GetReceivedSummary`: pendiente por mes, `due_this_month` y `overdue_this_month`.
- [ ] 7.2 `GetUpcoming` y `GET /api/v1/payments/upcoming`, con controller, ruta y godoc.
- [ ] 7.3 Tests del service y del controller (200, 401, 500).

## 8. Verificación y documentación

- [ ] 8.1 `EXPLAIN ANALYZE` de las consultas nuevas contra testing; índice solo si hace falta (design D7).
- [ ] 8.2 Regenerar swagger. Actualizar README y `docs/DEUDA_TECNICA_Y_PENDIENTES.md`.
- [ ] 8.3 `go vet ./...`, `go test ./...` y validar el change.
