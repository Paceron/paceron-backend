## 1. Change de OpenSpec

- [x] 1.1 Crear `proposal.md`, `design.md`, `specs/pagos-y-cobros-entrenador/spec.md` y `tasks.md`.

## 2. Constantes de estado de pago

- [x] 2.1 Crear `cmd/api/domains/constants/payment_status.go` con los estados de Mercado Pago, los grupos (`approved`, `pending`, `rejected`, `refunded`), `GroupOfPaymentStatus()` y `StatusesForGroup()`.
- [x] 2.2 Tests en `payment_status_test.go`.

## 3. DTOs e índice

- [x] 3.1 Crear `cmd/api/domains/payment/payment_history.go` con los DTOs del contrato.
- [x] 3.2 Agregar `index` a `SellerUserID` en `cmd/api/domains/dbs/payment.go`.

## 4. DAO de cobros recibidos

- [x] 4.1 Crear `cmd/api/daos/payment_history_dao.go` con `ListReceived` (paginado, filtros por equipo y estado) y `ListReceivedSince` (sin paginar, para el resumen).
- [x] 4.2 Excluir filas con `payment_id` vacío y extraer el neto real de `raw_response`.
- [x] 4.3 Tests contra Postgres: filtro por vendedor y concepto, filas sin `payment_id`, neto de webhook y de `ProcessPayment`, filtros, orden y `has_more`, error de DB.

## 5. DAO de pagos de tier

- [x] 5.1 Agregar `ListMyTierPayments` al mismo DAO (JOIN a `installments.subscription_id`, filtro opcional por rol).
- [x] 5.2 Tests: pago `order` con cuota de tier, cuota de equipo excluida, filtro por rol, error de DB.

## 6. Service de listados

- [ ] 6.1 Crear `cmd/api/services/payment_history_service.go` con `ListReceived` y `ListMyTierPayments`: validación de parámetros y mapeo a DTO.
- [ ] 6.2 Tests con mock del DAO.

## 7. Resumen mensual

- [ ] 7.1 Agregar `GetReceivedSummary`: meses en hora argentina, totales por equipo y conteo por último intento de cada cuota.
- [ ] 7.2 Tests: mes vacío, borde de huso, rechazo seguido de aprobado, neto parcial, ventana fuera de rango, error del DAO.

## 8. Controller

- [ ] 8.1 Crear `cmd/api/controllers/payment_history_controller.go` con los tres handlers y su godoc.
- [ ] 8.2 Tests: 200, 400, 401 y 500.

## 9. Wiring, rutas y swagger

- [ ] 9.1 Wiring en `cmd/api/app/app.go` y rutas en `cmd/api/app/url_mappings.go`.
- [ ] 9.2 Regenerar swagger con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.

## 10. Deuda técnica y cierre

- [ ] 10.1 Registrar en `docs/DEUDA_TECNICA_Y_PENDIENTES.md` la lectura del webhook con el token de la plataforma y las filas sin `payment_id`.
- [ ] 10.2 `go vet ./...`, `go test ./...` y `make coverage-with-db` en 85% o más.
- [ ] 10.3 Archivar el change después del merge.
