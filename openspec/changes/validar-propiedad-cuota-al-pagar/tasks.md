## 1. Constantes de error tipificadas

- [ ] 1.1 Agregar `PAYMENT_INSTALLMENT_NOT_FOUND` y `PAYMENT_INSTALLMENT_FORBIDDEN` en `cmd/api/domains/constants/error_code.go`.

## 2. Validación de propiedad en el service

- [ ] 2.1 En `paymentService.ProcessPayment`, cuando `req.InstallmentID != nil`, validar la cuota antes de contactar a Mercado Pago:
      - `installDao.FindByID` con error → propagar error (500 por infraestructura).
      - cuota `nil` (no existe) → error de recurso `PAYMENT_INSTALLMENT_NOT_FOUND`.
      - sin identidad autenticada (`utils.GetAuthUserID` sin ok) → error de auth/forbidden.
      - `installment.UserID != userID` → error `PAYMENT_INSTALLMENT_FORBIDDEN`.
- [ ] 2.2 Asegurar que los pagos de concepto `order` (sin `installment_id`) no tocan esta validación.

## 3. Controller: traducir errores a HTTP

- [ ] 3.1 En `paymentController.ProcessPayment`, traducir `PAYMENT_INSTALLMENT_NOT_FOUND` → 404 y `PAYMENT_INSTALLMENT_FORBIDDEN` → 403 (mismo formato `apierror.APIError`); el resto sigue como 500.

## 4. Tests

- [ ] 4.1 Service: dejar un `mockInstallmentDao` y casos para cuota inexistente (not found), cuota ajena (forbidden) y cuota propia (avanza al flujo normal).
- [ ] 4.2 Service: confirmar que un pago `order` sin `installment_id` no requiere `FindByID` (no rompe casos actuales).
- [ ] 4.3 Controller: meter el `auth_user_id` en el contexto y cubrir not-found (404) y forbidden (403).
- [ ] 4.4 Correr `go test ./...` y la suite completa en verde.

## 5. Swagger y cierre

- [ ] 5.1 Regenerar swagger (`make swagger` según README) si las anotaciones del controller cambian.
- [ ] 5.2 Commit del cambio (Conventional Commits) y push a la rama feature. El change OpenSpec queda listo para archivar tras el merge.