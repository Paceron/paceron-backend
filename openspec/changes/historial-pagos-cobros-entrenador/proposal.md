# Propuesta: Historial de pagos y cobros del entrenador

## Why

Un entrenador no tiene forma de ver su dinero dentro de Paceron. No puede
consultar los pagos que hizo por su suscripción de tier ni lo que le pagaron
los corredores por pertenecer a sus equipos. Los datos ya existen en
`payments` e `installments`, pero **no hay ningún endpoint que liste pagos**:
el `PaymentDao` solo tiene altas, actualizaciones y búsquedas puntuales.

El frontend necesita esta información para una sección "Pagos y cobros" en el
perfil del entrenador (ver `docs/BACKEND_PAYMENTS_REQUIREMENTS.md` del repo
frontend, donde la ubicación ya estaba resuelta).

## Objetivo

Que un usuario autenticado pueda consultar:

1. Los cobros de membresía de equipo que recibió como entrenador.
2. Sus propios pagos de suscripción de tier.
3. Un resumen mensual de sus cobros, con totales por equipo y la cantidad de
   cuotas pendientes o rechazadas.

Todo sin inventar comisiones: el monto bruto siempre, y el neto solo cuando
Mercado Pago lo informó de verdad.

## Alcance

- Tres endpoints `GET` autenticados:
  - `/api/v1/payments/received`
  - `/api/v1/payments/received/summary`
  - `/api/v1/payments/mine`
- Constantes de estado de pago de Mercado Pago y su agrupación.
- Un DAO de solo lectura con la extracción del neto real desde `raw_response`
  (jsonb).
- Un service con la paginación y la agregación del resumen.
- Controller, wiring en `app.go`, rutas y swagger.
- Índice sobre `payments.seller_user_id`.
- Tests de DAO, service y controller.
- Registro de la deuda técnica detectada en `docs/DEUDA_TECNICA_Y_PENDIENTES.md`.

## No alcance

- Arreglar que el webhook lea con el token de la plataforma los pagos creados
  con el token del entrenador.
- Enviar `application_fee` a Mercado Pago, o exponer `marketplace_fee` (el valor
  guardado es ficticio mientras el split no se envíe).
- Evitar que `CreatePreference` deje filas sin `payment_id`: acá solo se filtran.
- Cuotas adeudadas que todavía no tienen ningún intento de pago.
- Pagos que un corredor hace a sus equipos (su propio historial).
- Exportación (CSV, PDF).
- Guardar la fecha real de aprobación del pago.

## Métrica de éxito

- Los totales de `/payments/received/summary` coinciden peso a peso con la
  consulta SQL de control sobre la base de testing, para al menos un entrenador
  con cobros.
- Ningún resultado incluye filas con `payment_id` vacío.
- `make coverage-with-db` se mantiene en 85% o más, y los archivos nuevos
  quedan en 90% o más.

## What Changes

- Nuevo `PaymentHistoryDao` de solo lectura, sin tocar `PaymentDaoInterface`.
- Nuevo `PaymentHistoryService`, que depende únicamente de ese DAO.
- Nuevo `PaymentHistoryController` con tres handlers.
- Nuevas constantes `PaymentStatus*` y `PaymentStatusGroup*`.
- DTOs nuevos en `domains/payment/payment_history.go`.
- `dbs.Payment.SellerUserID` pasa a tener índice.
- **No rompe la API**: no cambia ningún endpoint existente.

## Capabilities

### New Capabilities

- `pagos-y-cobros-entrenador`: consulta paginada de cobros recibidos y de pagos
  de tier propios, más un resumen mensual de cobros.

### Modified Capabilities

- (ninguna)

## Impact

- **Schema**: índice nuevo `idx_payments_seller_user_id`, creado por AutoMigrate.
- **Modelos**: `dbs.Payment` (solo el tag del índice).
- **DTOs**: `domains/payment/payment_history.go` (nuevo).
- **Constantes**: `domains/constants/payment_status.go` (nuevo).
- **DAOs**: `daos/payment_history_dao.go` (nuevo).
- **Servicios**: `services/payment_history_service.go` (nuevo).
- **Controllers**: `controllers/payment_history_controller.go` (nuevo).
- **API**: tres rutas nuevas en `app/url_mappings.go`, bloque de pagos
  autenticados.
- **Swagger**: regenerado.
- **Tests**: DAO (Postgres real), service (mocks) y controller (`httptest`).
- **Dependencias**: ninguna nueva.
