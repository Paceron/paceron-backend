# Diseño: Validar la propiedad de la cuota al procesar un pago

## Context

`POST /api/v1/payments` (`paymentService.ProcessPayment`) cobra una cuota de
suscripción vía Mercado Pago. El front manda un `installment_id` (opcional; para
pagos simples `order` no se usa). Hoy el backend **confía** en ese id sin
verificar que la cuota exista ni que sea del usuario autenticado. Eso permite:

- **IDOR**: pagar y activar la cuota de otra persona si se conoce su
  `installment_id`.
- **Inconsistencia**: cobrar a MP una cuota inexistente; el pago aprueba y
  después el webhook no encuentra la cuota y no se confirma (dinero cobrado sin
  activar nada).

La cuota (`dbs.Installment`) guarda su dueño en `user_id`, tanto para
suscripciones de tier (`subscription_id`) como de equipo (`team_id`). La identidad
del usuario autenticado está en el contexto de Gin (`utils.GetAuthUserID`).

## Goals / Non-Goals

**Goals:**
- Rechazar temprano (antes de llamar a MP) los `installment_id` inexistentes o que
  no pertenezcan al usuario autenticado.
- Aplicar la validación **uniformemente** a cuotas de tier y de equipo, sin
  duplicar lógica.
- No tocar el flujo de confirmación por webhook (sigue siendo confiable e
  idempotente).
- Mantener el contrato del request intacto (solo se suman errores).

**Non-Goals:**
- No proteger `CreatePreference` (crear una preferencia no mueve dinero; quedará
  como mejora futura si aplica).
- No hacer migraciones de datos ni cambios de modelo: `Installment.user_id` ya
  existe y es `not null`.

## Decisions

### D1 — Validar en `ProcessPayment`, no en el controller
La validación vive en el **service** (`paymentService.ProcessPayment`), siguiendo
la regla de capas: el controller es delgado y la lógica de negocio va en el
service. El service ya recibe `*gin.Context` (patrón del repo), así que lee la
identidad con `utils.GetAuthUserID(ctx)` sin cambiar la firma del método.

- Alternativa descartada: validar en el controller. Dejaría lógica de negocio en
  la capa HTTP y duplicaría el chequeo si otro flujo llama al service.

### D2 — Chequeo `FindByID` + comparación de `user_id` (uniforme tier/equipo)
Al entrar con `req.InstallmentID != nil`:
1. `installDao.FindByID(ctx, *req.InstallmentID)`.
   - error → `500` (falla de infraestructura).
   - `nil` (no existe) → `PAYMENT_INSTALLMENT_NOT_FOUND` (404).
2. `id, ok := utils.GetAuthUserID(ctx)`; si no hay identidad →
   `PAYMENT_INSTALLMENT_FORBIDDEN`-like rechazo (pedirá auth).
3. `installment.UserID != id` → `PAYMENT_INSTALLMENT_FORBIDDEN` (403).

Como `Installment.user_id` está seteado en ambos tipos (tier y equipo), **un solo
`FindByID` y una comparación** cubren el caso; no hace falta resolver la
suscripción ni el `team_user`.

- Alternativa descartada: validar a través de la suscripción (`subscription.UserID`)
  para tier y del `team_user` para equipo. Implica consultas extra y ramificar por
  tipo; `Installment.user_id` ya es la fuente única de verdad del dueño.

### D3 — La validación es condicional a `InstallmentID != nil`
Los pagos de concepto `order` (sin cuota) no llevan `installment_id`; esos casos
no deben tocar el `FindByID` ni requerir identidad de cuota. Solo cuando el front
paga una **cuota de suscripción** se valida propiedad. Esto es compatible con los
tests existentes de `ProcessPayment` (que no usan `installment_id`).

### D4 — Nuevos códigos de error en `constants/error_code.go`
- `PAYMENT_INSTALLMENT_NOT_FOUND` → `404`.
- `PAYMENT_INSTALLMENT_FORBIDDEN` → `403`.
Siguen el patrón tipificado ya usado (`TIER_NOT_FOUND`, `DEBT_BLOCKS_OPERATION`).
El `paymentController.ProcessPayment` traduce estos códigos a las respuestas HTTP
con `apierror.APIError` (mismo formato que el resto de la app); cualquier otro
error sigue como `500`.

### D5 — No cambiar la firma del servicio (evita cascada)
`ProcessPayment(ctx, req)` se mantiene. El user id sale del contexto, con un
helper ya existente (`utils.GetAuthUserID`), sin tocar `PaymentServiceInterface`,
delegates ni app.go. Menos superficie de cambio = menos riesgo.

## Risks / Trade-offs

- **Riesgo: romper pagos de cuota legítimos si la cuota se crea con `user_id` nulo**
  → `user_id` es `not null` en `installments`; los generadores
  (`FirstInstallment`, `BuildNextInstallment`) siempre lo setean. Verificado en
  `installment_engine.go`.
- **Riesgo: el IDOR del equipo/split** — el `team_user` dueño puede no ser el que
  pagó. → El `installment.user_id` de una membresía es el miembro (`NextTeamInstallment`
  usa `tu.UserID`), por lo que el que paga su cuota es su dueño; correcto.
- **Riesgo: coste de una query extra por pago** → una sola `FindByID` indexada por
  PK; insignificante y solo en el camino de cuotas.
- **Riesgo: tests existentes que no mockean `installDao`** → la validación es
  condicional a `InstallmentID != nil`; los tests actuales sin cuota no la
  disparan. Los tests nuevos inyectan un `mockInstallmentDao`.
- **Trade-off: no toca `CreatePreference`** → un atacante podría crear preferencias
  de cuotas ajenas, pero no cobrarlas si no puede completar el pago de la suya;
  la superficie de dinero en movimiento queda protegida.

## Migration Plan

- No aplica migración de datos. El cambio es de lógica en `ProcessPayment` +
  códigos de error nuevos.
- Rollback: revertir el commit del service/controller/constants; la API vuelve al
  comportamiento previo sin cambios de schema.

## Open Questions

- ¿Se quiere agregar la validación de propiedad también a `CreatePreference`
  (crear preferencia de una cuota ajena)? → Dejado fuera de alcance a propósito;
  puede seguirse en otro change si el equipo lo considera. (Ver Trade-off.)