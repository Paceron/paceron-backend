## ADDED Requirements

### Requirement: Envío de correos vía SMTP con cuenta Gmail
El sistema SHALL proveer un cliente de infraestructura capaz de enviar correos electrónicos HTML vía SMTP usando una cuenta de Gmail autenticada con usuario y contraseña de aplicación, sobre conexión STARTTLS en el puerto 587.

#### Scenario: Envío exitoso de un correo
- **WHEN** se invoca `Client.Send` con un destinatario, asunto y cuerpo HTML válidos, y las credenciales SMTP configuradas son correctas
- **THEN** el correo SHALL ser entregado al servidor SMTP de Gmail sin error

#### Scenario: Credenciales SMTP inválidas
- **WHEN** se invoca `Client.Send` con credenciales SMTP incorrectas
- **THEN** el sistema SHALL retornar un error describiendo la falla, sin hacer panic

#### Scenario: Configuración vía variables de entorno
- **WHEN** el sistema arranca con `GMAIL_USER`, `GMAIL_APP_PASSWORD`, `SMTP_HOST` y `SMTP_PORT` configurados en el entorno
- **THEN** `config.MySMTP` SHALL contener esos valores, con `SMTP_PORT` por defecto en 587 si no está seteado o es inválido

### Requirement: Template HTML de bienvenida parametrizado
El sistema SHALL proveer un template HTML embebido de correo de bienvenida, parametrizado con el nombre del usuario, usando los colores de marca de Paceron.

#### Scenario: Renderizado exitoso con nombre
- **WHEN** se invoca `RenderWelcomeEmail` con `WelcomeEmailData{ Name: "Juan" }`
- **THEN** el HTML resultante SHALL contener el nombre "Juan" interpolado en el saludo

#### Scenario: Auto-escaping de caracteres especiales
- **WHEN** se invoca `RenderWelcomeEmail` con un nombre que contiene caracteres HTML especiales
- **THEN** el HTML resultante SHALL tener esos caracteres escapados, sin permitir inyección de HTML/JS

### Requirement: Prueba end-to-end vía test automatizado
El sistema SHALL exponer un test que verifique el envío real de correos cuando hay credenciales disponibles, sin requerirlas para el resto de la suite.

#### Scenario: Envío real con credenciales configuradas
- **WHEN** se ejecuta `go test ./cmd/api/infrastructure/mailer/...` con las variables de entorno SMTP configuradas con credenciales reales
- **THEN** el test SHALL enviar un correo real y verificar que no hay error

#### Scenario: Test se saltea sin credenciales
- **WHEN** se ejecuta `go test ./...` en una máquina sin las variables de entorno SMTP configuradas
- **THEN** el test de envío real SHALL saltearse automáticamente (SKIP) y el resto de la suite SHALL pasar sin error

### Requirement: Envío de email de bienvenida al registrar un usuario
El sistema SHALL intentar enviar un email de bienvenida al usuario inmediatamente después de un registro exitoso (`POST /api/v1/auth/register`), sin que un fallo de envío afecte la respuesta del registro.

#### Scenario: Registro exitoso dispara el envío
- **WHEN** un usuario se registra exitosamente con datos válidos
- **THEN** el sistema SHALL invocar `SendWelcomeEmail` con el email y nombre del usuario recién creado

#### Scenario: Fallo de envío no bloquea el registro
- **WHEN** un usuario se registra exitosamente pero el envío del email de bienvenida falla (ej. credenciales SMTP inválidas o Gmail no disponible)
- **THEN** el sistema SHALL responder igualmente HTTP 201 con los datos del usuario creado, y SHALL loguear el error de envío sin exponerlo en la respuesta
