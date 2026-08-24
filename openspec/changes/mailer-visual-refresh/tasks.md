## 1. Assets

- [x] 1.1 Generar 8 íconos de acento (badge circular verde `#8cc63e` + glifo oscuro `#111518`) desde Material Community Icons (`@expo/vector-icons`, mismo font que usa `paceron-frontend`), vía ImageMagick, 176x176px
- [x] 1.2 Mapeo ícono/evento: `hand-wave` (welcome), `logout-variant` (farewell), `lock-reset` (password_reset), `shield-check-outline` (password_changed), `email-fast-outline` (invitation), `email-check-outline` (invitation_response), `account-remove-outline` (team_removed), `account-arrow-right-outline` (team_member_left)
- [x] 1.3 Guardar en `cmd/api/infrastructure/mailer/assets/icon-*.png`

## 2. Backend

- [x] 2.1 `render.go`: `//go:embed` de los 8 íconos en `iconAssets embed.FS`, `eventIconPaths map[EmailType]string`, const `eventIconContentID`
- [x] 2.2 `mailer.go`: refactor de `Send`/`SendEmail` — método privado `send` compartido; `SendEmail` resuelve y adjunta el ícono de evento vía `eventIconAttachment` (best-effort: un ícono faltante no impide el envío), `Send` sigue mandando solo el logo (no conoce el `EmailType`)
- [x] 2.3 `MailerInterface` sin cambios — mismo contrato público

## 3. Templates (8)

- [x] 3.1 Header: banda verde sólida → fondo blanco, banda ancha (`cid:header-mark`, 380x43, ver sección 6) + ícono de evento (`cid:event-icon`, 64x64) debajo, centrados
- [x] 3.2 Card: fondo de página `#f0f0f0`, card blanca con `border:1px solid #e2e8f0`, `border-radius:16px`, `box-shadow`
- [x] 3.3 Tipografía: títulos a `font-weight:700`, cuerpo con `line-height:23px`
- [x] 3.4 Footer con fondo `#fafafa` sutil
- [x] 3.5 `reset.html`: caja del código adaptada al nuevo fondo blanco de la card (`background-color:#f0f0f0` en vez de `#ffffff`, para que siga contrastando)
- [x] 3.6 Copy de cada template sin cambios — solo el wrapper visual
- [x] 3.7 `<meta name="color-scheme" content="light">` + `<style>:root{color-scheme:light only}</style>` en los 8 — intento de forzar tema claro sin importar el cliente (ver sección 6, resultado parcial)

## 4. Tests

- [x] 4.1 `TestRenderEmail_Welcome`/`TestRenderEmail_Farewell`: assertion de `#8cc63e` inline (header viejo) reemplazada por `cid:event-icon`
- [x] 4.2 `TestSendEmail_AttachesEventIcon`: contra servidor real (`httptest`), verifica 2 attachments (logo + ícono) con los `content_id` esperados
- [x] 4.3 `TestSend_OnlyAttachesLogo`: `Send` genérico sigue mandando un solo attachment
- [x] 4.4 `go build`/`go vet`/`go test ./...` verdes

## 5. Verificación manual (hecha en conjunto durante la sesión)

- [x] 5.1 Reenviados varios tipos de correo con credenciales reales de Resend contra un destinatario real (Gmail), confirmado visualmente en cliente real (no solo preview de Chrome headless) — welcome, invitation, team_removed, password_reset
- [x] 5.2 Ícono y logo confirmados nítidos y bien alineados, tanto en Gmail web como en Gmail Android

## 6. Hallazgos durante la verificación manual (post-diseño inicial)

Iterando con envíos reales aparecieron 3 problemas que no existían en el plan original, todos resueltos o cerrados con decisión explícita:

- [x] 6.1 **Wordmark con inclinación invertida**: el primer intento reconstruía el wordmark desde la tipografía (`Orbitron_700Bold` + shear vía ImageMagick) en vez de usar el asset de marca real — la dirección del shear no coincidía con la marca real y se perdía el acento verde de la "a". Resuelto usando directamente los archivos de marca reales que proveyó el usuario (símbolo y wordmark separados, alta resolución) en vez de recrear tipografía.
- [x] 6.2 **Las imágenes embebidas (CID) aparecían como "adjunto" en la lista de Gmail**: investigado a fondo (doc de Resend, issues de GitHub, foros de Gmail) — no hay campo de `disposition` en la API de Resend, es indistinguible desde el lado del remitente. Se resolvió empíricamente: el logo pesaba ~45KB con nombre/`content_id` conteniendo la palabra "logo"; renombrando a `header-mark` + optimizando a paleta de 32 colores (PNG8, ~5KB, más liviano que los íconos) el chip dejó de aparecer. No confirmado el mecanismo exacto (nombre vs. peso vs. combinación), pero el resultado es reproducible.
- [x] 6.3 **Gmail Android fuerza dark mode y no respeta `color-scheme`**: confirmado con fuente externa (Litmus) que Gmail Android usa un "partial invert" propio sin soporte de `@media (prefers-color-scheme)` — no hay técnica CSS que lo evite de forma confiable. Decisión explícita del usuario: aceptar que el cuerpo del mail se adapte al tema del cliente (sigue siendo legible en ambos casos), no perseguir hacks riesgosos (`mix-blend-mode`) que además van en contra de la preferencia de accesibilidad del usuario. Como mitigación cosmética, el header (`header-mark`) se ensanchó para ocupar más ancho de la card y funcionar como banda clara reconocible en vez de una pill chica perdida en dark mode.
