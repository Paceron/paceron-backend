# Spec Delta: mp-connect

> Cambio: `optimizaciones-y-seguridad-api`. Reemplaza/actualiza el `oauth-state` de la capability de MP Connect (capability no archivada — declarado completo bajo ADDED por convención del repo).

## ADDED Requirements

### Requirement: State de OAuth firmado

El sistema DEBE cumplir lo siguiente (MUST):

- El `state` del flujo OAuth de MP Connect es un payload firmado (HMAC-SHA256 con secret del server) sobre `userID-timestamp-target`; la firma viaja junto al payload y se verifica ANTES de parsear.
- Un state con firma inválida (forjado o manipulado) es rechazado en el callback con el flujo de error existente — nunca vincula la cuenta MP de nadie.
- El chequeo de antigüedad del state se mantiene.

#### Scenario: State forjado rechazado

- **WHEN** el callback recibe un state con payload bien formado pero firma inválida (no emitida por el server)
- **THEN** el callback es rechazado con el error existente y NO se vincula ninguna cuenta de Mercado Pago

#### Scenario: State legítimo sin fricción

- **WHEN** el callback recibe un state firmado y vigente emitido por el server
- **THEN** la vinculación sigue como hoy (shape HTTP sin cambios)

### Requirement: Validación de firma del webhook de deauth

El sistema DEBE cumplir lo siguiente (MUST):

- Si el secret de webhook está configurado, `POST /mercadopago/webhook/connect` valida la firma de la notificación con el mismo mecanismo del webhook de pagos; inválida → `401` sin aplicar la desautorización.
- Sin secret configurado, mantiene el comportamiento actual + warning (rollout gradual).

#### Scenario: Deauth con firma válida aplica

- **WHEN** la notificación de deauth llega firmada por MP y el secret está configurado
- **THEN** la desautorización se aplica igual que hoy

#### Scenario: Deauth forjada rechazada

- **WHEN** un POST forjado intenta desautorizar a un entrenador con el secret configurado
- **THEN** recibe `401` y la conexión del entrenador no se toca
