## MODIFIED Requirements

### Requirement: Envío de correos vía la API HTTP de Resend
El sistema SHALL proveer un cliente de infraestructura capaz de enviar correos electrónicos HTML vía la API HTTP de Resend, autenticado con un API key vía header `Authorization: Bearer`, reusando `infrastructure/httpclient` para timeout y reintentos.

#### Scenario: Envío exitoso de un correo
- **WHEN** se invoca `Client.Send` con un destinatario, asunto y cuerpo HTML válidos, y el API key configurado es correcto
- **THEN** el correo SHALL ser entregado a la API de Resend sin error, con el logo de Paceron embebido como attachment inline (`content_id`)

#### Scenario: API key inválido
- **WHEN** se invoca `Client.Send` con un API key incorrecto o revocado
- **THEN** el sistema SHALL retornar un error describiendo la falla, sin hacer panic

#### Scenario: Configuración vía variables de entorno
- **WHEN** el sistema arranca con `RESEND_API_KEY` y `RESEND_FROM_ADDRESS` configurados en el entorno
- **THEN** `config.MyMailer` SHALL contener esos valores

#### Scenario: Falta el API key o el remitente al construir el cliente
- **WHEN** se invoca `mailer.New` sin `WithAPIKey` o sin `WithFrom`
- **THEN** el sistema SHALL retornar un error explícito, sin construir un cliente parcialmente configurado

### Requirement: Prueba end-to-end vía test automatizado
El sistema SHALL exponer un test que verifique el envío real de correos vía Resend cuando hay credenciales disponibles, sin requerirlas para el resto de la suite.

#### Scenario: Envío real con credenciales configuradas
- **WHEN** se ejecuta `go test ./cmd/api/infrastructure/mailer/...` con `RESEND_API_KEY`/`RESEND_FROM_ADDRESS` configurados con credenciales reales
- **THEN** el test SHALL enviar un correo real de cada `EmailType` y verificar que no hay error

#### Scenario: Test se saltea sin credenciales
- **WHEN** se ejecuta `go test ./...` en una máquina sin `RESEND_API_KEY` configurada
- **THEN** el test de envío real SHALL saltearse automáticamente (SKIP) y el resto de la suite SHALL pasar sin error

## RENAMED Requirements
- FROM: `### Requirement: Envío de correos vía SMTP con cuenta Gmail`
- TO: `### Requirement: Envío de correos vía la API HTTP de Resend`
