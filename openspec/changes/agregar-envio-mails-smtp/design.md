## Context

El backend de Paceron no tenía ninguna capacidad de envío de correos electrónicos. Se necesitaba esta infraestructura para cumplir el criterio de aceptación de la historia de registro de usuario ("email de bienvenida"), construyendo primero la capacidad de forma aislada y luego conectándola al flujo real de `POST /api/v1/auth/register`.

## Goals / Non-Goals

**Goals:**
- Cliente SMTP reutilizable en `infrastructure/mailer/`, siguiendo el patrón de opciones funcionales de `infrastructure/httpclient/`
- Envío de correos HTML usando una cuenta de Gmail (usuario + contraseña de aplicación) vía STARTTLS (puerto 587)
- Template HTML de bienvenida embebido en el binario (`go:embed`), con los colores de marca de Paceron, parametrizado con el nombre del usuario
- Configuración de SMTP vía variables de entorno, siguiendo el patrón existente de `config.go`
- Conectar el envío del email de bienvenida al flujo de registro de usuarios, sin que un fallo de envío bloquee el alta

**Non-Goals:**
- Conectar a otros flujos (recuperación de contraseña, notificaciones varias) — solo registro en este cambio
- Envío de correos en batch/cola/retry asíncrono
- Múltiples templates (solo el de bienvenida en este cambio)
- Soporte de adjuntos, CC/BCC, u otras features avanzadas de correo
- Pruebas unitarias dedicadas al flujo `Register + mailer` (mocks de `MailerInterface`, casos de error específicos) — se mantiene la suite existente compilando y en verde, pero la cobertura nueva queda para una iteración posterior

## Decisions

### 1. Librería SMTP: github.com/wneessen/go-mail
- **Por qué**: Librería moderna, mantenida activamente, sin dependencias externas (solo stdlib de Go), con soporte nativo de STARTTLS y autenticación compatible con Gmail. Evita tener que armar manualmente el MIME multipart para HTML.
- **Alternativa**: `net/smtp` + armado manual de MIME — descartada porque requiere construir a mano los boundaries multipart, headers y encoding, una fuente común de bugs sutiles, sin beneficio real dado que `go-mail` no agrega dependencias transitivas.

### 2. Ubicación del código: infrastructure/mailer/, no restclients/
- **Por qué**: SMTP no es HTTP/REST, por lo que no pertenece a `restclients/` (que está reservado para clientes de APIs externas sobre `httpclient`). `infrastructure/` es la carpeta correcta para "herramientas transversales reutilizables" que conectan con sistemas externos, tal como ya lo son `postgresdb/` y `httpclient/`.
- **Alternativa**: crear `restclients/gmailclient/` — descartada, violaría la regla explícita de que `restclients/` es solo para HTTP.

### 3. Puerto 587 con STARTTLS (TLSMandatory), no 465 SSL
- **Por qué**: 587 + STARTTLS es el estándar moderno recomendado para envío autenticado, y Gmail lo soporta explícitamente. `TLSMandatory` fuerza que la conexión falle ruidosamente si el servidor no soporta STARTTLS, en vez de degradar silenciosamente a texto plano.
- **Alternativa**: 465 SSL — descartada.

### 4. Renderizado de templates: html/template + go:embed, en archivo separado (render.go)
- **Por qué**: `html/template` (no `text/template`) porque el output es HTML y necesitamos auto-escaping para evitar inyección si el nombre de un usuario contiene caracteres especiales. Se separa en `render.go` (vs. meterlo en `mailer.go`) porque son responsabilidades distintas: `mailer.go` se encarga del transporte SMTP, `render.go` del parseo/renderizado de templates — mismo criterio de un archivo por responsabilidad que ya usa `infrastructure/httpclient/`.
- **Alternativa**: todo en un solo `mailer.go` — descartada por mezclar dos responsabilidades que van a crecer de forma independiente (más templates en el futuro).

### 5. Estructura del template HTML: tabla, CSS inline, colores de marca
- **Por qué**: Los clientes de correo (Gmail, Outlook, Apple Mail) no soportan CSS externo/`<style>` de forma confiable, por lo que se usa layout basado en tablas anidadas con estilos inline. Los colores usados son los de la marca (verde `#8cc63e`, texto `#111518`, texto secundario `#40484c`, fondo `#ffffff`, contenedor `#f0f0f0`), extraídos de `paceron-frontend/theme/colors.js` (modo claro).

### 6. Conexión al registro: envío best-effort, nunca bloquea el alta
- **Por qué**: El email de bienvenida es una notificación, no un requisito para que la cuenta exista. Si Gmail está caído, las credenciales son inválidas, o hay un problema de red, el usuario igual debe quedar registrado. `authService.Register` loguea el error con `customlogger.Error` (mismos tags `email`/`step` que el resto del archivo) pero siempre retorna la respuesta exitosa si la creación del usuario en DB funcionó.
- **Alternativa**: hacer el envío síncrono y bloqueante (si falla el mail, falla el registro) — descartada, acopla la disponibilidad de un servicio externo (Gmail) a una operación crítica de negocio (alta de usuario).

### 7. Mailer nil-safe en el service, sin mocks nuevos todavía
- **Por qué**: `authService.mailer` es una interfaz que puede ser `nil` (chequeada antes de usarse). Esto permite que los 17 tests existentes de `auth_service_test.go` seguían compilando pasando `nil` como segundo argumento de `NewAuthService`, sin necesidad de escribir un mock de `MailerInterface` en esta pasada.
- **Alternativa**: escribir un mock completo de `MailerInterface` con aserciones de que fue invocado — queda para la iteración de pruebas unitarias dedicadas.

## Risks / Trade-offs

- **Contraseña de aplicación de Gmail en variables de entorno** → Mitigación: nunca comitear `.env` (ya está en `.gitignore`), documentar en `.env.example` solo como placeholder vacío
- **Límites de envío de Gmail** (cuentas gratuitas tienen límite diario de ~500 correos) → Aceptable para este alcance; documentar como limitación conocida si el volumen de registros crece
- **Sin reintentos automáticos en el mailer** → Decisión consciente: a diferencia de `httpclient` (que tiene retry + circuit breaker), el mailer no los implementa en este cambio; se puede agregar si se vuelve necesario
- **Cobertura de tests del flujo Register+mailer diferida** → Mitigación: el nil-check evita romper la suite existente; la cobertura específica (mocks, casos de error de envío) se planifica como trabajo de seguimiento explícito, no se pierde de vista
