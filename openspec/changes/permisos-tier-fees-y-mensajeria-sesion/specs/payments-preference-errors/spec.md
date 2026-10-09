# payments-preference-errors delta

## ADDED Requirements

### Requirement: Códigos de error reales en POST /payments/preference

El sistema DEBE cumplir lo siguiente (MUST):

- `POST /payments/preference` responde el código HTTP real del problema, con body `{"status_code":N,"code":"...","message":"..."}`:
  - `409` con `code:"SELLER_NOT_CONNECTED"` cuando el entrenador dueño del equipo no tiene conexión Mercado Pago authorized (mensaje actual: "el entrenador debe conectar su cuenta de Mercado Pago").
  - `404` cuando la cuota o el equipo referenciados no existen.
  - `400` para las validaciones de entrada que ya realiza el controller/service.
- Los errores restantes (upstream de Mercado Pago, DAO, no identificables) mantienen `500` con el mensaje genérico actual — sin 502.
- La respuesta `201` de creación exitosa no cambia.

#### Scenario: Entrenador sin Mercado Pago conectado

- **WHEN** el corredor pide una preferencia para una cuota de team_subscription y el owner no tiene MP conectado
- **THEN** responde `409` con `code:"SELLER_NOT_CONNECTED"` en el body JSON.

#### Scenario: Cuota inexistente

- **WHEN** el `installment_id` referenciado no existe
- **THEN** responde `404`.

#### Scenario: Causa no identificable

- **WHEN** falla una llamada upstream a Mercado Pago por disponibilidad
- **THEN** responde `500` con el mensaje genérico de siempre (el frontend muestra su mensaje vago).
