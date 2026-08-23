## 1. Assets

- [x] 1.1 Generar 8 íconos de acento (badge circular verde `#8cc63e` + glifo oscuro `#111518`) desde Material Community Icons (`@expo/vector-icons`, mismo font que usa `paceron-frontend`), vía ImageMagick, 176x176px
- [x] 1.2 Mapeo ícono/evento: `hand-wave` (welcome), `logout-variant` (farewell), `lock-reset` (password_reset), `shield-check-outline` (password_changed), `email-fast-outline` (invitation), `email-check-outline` (invitation_response), `account-remove-outline` (team_removed), `account-arrow-right-outline` (team_member_left)
- [x] 1.3 Guardar en `cmd/api/infrastructure/mailer/assets/icon-*.png`

## 2. Backend

- [x] 2.1 `render.go`: `//go:embed` de los 8 íconos en `iconAssets embed.FS`, `eventIconPaths map[EmailType]string`, const `eventIconContentID`
- [x] 2.2 `mailer.go`: refactor de `Send`/`SendEmail` — método privado `send` compartido; `SendEmail` resuelve y adjunta el ícono de evento vía `eventIconAttachment` (best-effort: un ícono faltante no impide el envío), `Send` sigue mandando solo el logo (no conoce el `EmailType`)
- [x] 2.3 `MailerInterface` sin cambios — mismo contrato público

## 3. Templates (8)

- [x] 3.1 Header: banda verde sólida → fondo blanco, logo apilado (`cid:paceron-logo`, 184x32) + ícono de evento (`cid:event-icon`, 64x64) debajo, centrados
- [x] 3.2 Card: fondo de página `#f0f0f0`, card blanca con `border:1px solid #e2e8f0`, `border-radius:16px`, `box-shadow`
- [x] 3.3 Tipografía: títulos a `font-weight:700`, cuerpo con `line-height:23px`
- [x] 3.4 Footer con fondo `#fafafa` sutil
- [x] 3.5 `reset.html`: caja del código adaptada al nuevo fondo blanco de la card (`background-color:#f0f0f0` en vez de `#ffffff`, para que siga contrastando)
- [x] 3.6 Copy de cada template sin cambios — solo el wrapper visual

## 4. Tests

- [x] 4.1 `TestRenderEmail_Welcome`/`TestRenderEmail_Farewell`: assertion de `#8cc63e` inline (header viejo) reemplazada por `cid:event-icon`
- [x] 4.2 `TestSendEmail_AttachesEventIcon`: contra servidor real (`httptest`), verifica 2 attachments (logo + ícono) con los `content_id` esperados
- [x] 4.3 `TestSend_OnlyAttachesLogo`: `Send` genérico sigue mandando un solo attachment
- [x] 4.4 `go build`/`go vet`/`go test ./...` verdes

## 5. Verificación manual (a cargo del usuario)

- [ ] 5.1 Reenviar los 8 tipos de correo con credenciales reales de Resend (local), confirmar visualmente en un cliente de correo real (no solo el preview de Chrome headless usado durante el desarrollo)
- [ ] 5.2 Confirmar que los 8 íconos se ven nítidos y bien alineados en al menos un cliente de correo mobile y uno de escritorio
