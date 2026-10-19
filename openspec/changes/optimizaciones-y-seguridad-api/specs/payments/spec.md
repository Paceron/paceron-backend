# Spec Delta: payments

> Cambio: `optimizaciones-y-seguridad-api`. Reemplaza/actualiza `ownership` y `webhook-auth` sobre la capability de pagos (capability no archivada — declarado completo bajo ADDED por convención del repo).

## ADDED Requirements

### Requirement: Ownership de consultas de pago

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /payments/:id` SOLO es legible por el pagador (`payment.UserID`) o el vendedor cobrador (`payment.SellerUserID`) del pago local. Cualquier otro usuario autenticado recibe `404`, sin filtrar existencia.
- `GET /payments/mp/:id` aplica el mismo guard de ownership resolviendo el pago local por `external_reference` ANTES de contactar Mercado Pago; sin ownership → `404` sin llamadas a MP.

#### Scenario: Ajeno no lee el pago de otro

- **WHEN** un usuario autenticado consulta `GET /payments/:id` de un pago que no pagó ni cobró
- **THEN** recibe `404` y no se expone ningún dato del pago (ni PII del payer)

#### Scenario: Payer y seller sí leen su pago

- **WHEN** el pagador o el vendedor consultan `GET /payments/:id`
- **THEN** reciben `200` con el shape actual sin cambios

#### Scenario: Consulta MP con ownership previo

- **WHEN** un ajeno consulta `GET /payments/mp/:id` de un pago ajeno
- **THEN** recibe `404` y NO se consume ninguna llamada HTTP a Mercado Pago

### Requirement: Validación de firma del webhook de pagos

El sistema DEBE cumplir lo siguiente (MUST):

- Si el secret de webhook de Mercado Pago está configurado, `POST /mercadopago/webhook` valida la firma `X-Signature` (formato `ts,v1=<hex>`) contra `X-Request-Id` y el id de la notificación con el validador HMAC existente; firma inválida o headers faltantes → `401` sin procesar el evento.
- Si el secret NO está configurado, el webhook mantiene el comportamiento actual (aceptar) y registra un warning — rollout gradual por env.
- La firma cruda NO se loguea completa.

#### Scenario: Firma válida procesa el evento

- **WHEN** Mercado Pago envía un webhook con firma válida y secret configurado
- **THEN** el evento se procesa igual que hoy (re-fetch, idempotencia, transiciones)

#### Scenario: Firma inválida rechazada

- **WHEN** un webhook llega con firma inválida (o headers faltantes) y secret configurado
- **THEN** responde `401` sin tocar base de datos ni llamar a MP

#### Scenario: Sin secret configurado no rompe

- **WHEN** el secret no está configurado y llega un webhook
- **THEN** se procesa como hoy y se registra warning
