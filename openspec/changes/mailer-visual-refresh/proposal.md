## Why

Los 8 templates de mail (`welcome`, `farewell`, `reset`, `invitation`, y los 4 nuevos de `push-notifications`) comparten un layout plano: card gris `#f0f0f0` con esquinas de 8px, sin sombra, header en banda verde sólida con el logo lado a lado. El usuario lo señaló como "estética pobre" al probar el envío real tras la migración a Resend. Se decidió mejorarlo tomando como referencia el lenguaje visual real de `paceron-frontend` — paleta (`theme/colors.js`), escala de espaciado/radio (`theme/tokens.js`) y el patrón de card usado en `auth-card-shell.jsx` — sin calcar ninguna pantalla puntual de la app, ya que el mail es su propia superficie con sus propias reglas (no hay botones con deep-link, eso queda para una iniciativa futura separada).

## What Changes

- **Header**: banda verde sólida → fondo blanco, logo apilado (ícono arriba, wordmark abajo, centrado) — mismo lockup que `auth-card-shell.jsx`, sin copiarlo literal (ahí el logo está dentro de una card con botón "Volver"; acá es solo el header).
- **Card contenedora**: fondo de página gris claro (`#f0f0f0`, `surfaceContainerHigh` de la app) por fuera, card blanca con `border` sutil + sombra suave + esquinas de 16px (antes 8px, sin sombra) — patrón "card flotando sobre fondo", no una réplica de un componente específico de la app.
- **Ícono de acento por tipo de evento**: cada uno de los 8 templates suma un badge circular (fondo verde `#8cc63e`, glifo oscuro `#111518`) debajo del logo, representando el evento — mismo mecanismo de asset que el logo (rasterizado a PNG desde Material Community Icons vía ImageMagick, embebido por Content-ID). Sin sistema semáforo de colores (ningún trigger es un error real, todos usan el mismo verde).
- **Tipografía y espaciado**: jerarquía más marcada (tamaños/pesos), espaciado alineado a la escala real de la app (4/8/12/16/20/24/32 vía `theme/tokens.js`) en vez de valores sueltos.
- **Explícitamente fuera de alcance**: botones con call-to-action / deep-link a la app — no existe la infraestructura (universal links) todavía, se evalúa como iniciativa aparte más adelante.

## Capabilities

### Modified Capabilities
- `smtp-mailer`: se agregan requerimientos sobre la presentación visual de los templates (antes no documentada) — mismo contrato de envío (`MailerInterface`, `EmailType`, `EmailData`), cambia solo el HTML renderizado y los assets embebidos.

## Impact

- **Nuevo**: 8 assets `cmd/api/infrastructure/mailer/assets/icon-*.png` (badge por tipo de evento).
- **Modificado**: los 8 templates HTML (`templates/*.html`), `mailer.go` (embeber el ícono correspondiente además del logo, por tipo de email).
- **Tests**: actualizar `TestRenderEmail_ReferencesEmbeddedLogo` y sumar equivalente para el ícono por tipo; `go build`/`go vet`/`go test ./...` verdes.
- **Verificación manual**: reenviar los 8 tipos con credenciales reales de Resend, confirmar visualmente en un cliente de correo real.
