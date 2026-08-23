## ADDED Requirements

### Requirement: Ícono de acento embebido por tipo de correo
El sistema SHALL embeber, además del logo de Paceron, un ícono de acento representando el evento en cada correo enviado con `SendEmail`, referenciado desde el HTML por Content-ID.

#### Scenario: Tipo de correo con ícono registrado
- **WHEN** se invoca `SendEmail` con un `EmailType` que tiene un ícono asociado en `eventIconPaths`
- **THEN** el correo SHALL incluir dos attachments inline (logo y el ícono del evento), cada uno con su propio `content_id`

#### Scenario: Falla la lectura del ícono
- **WHEN** el asset del ícono de un tipo registrado no puede leerse
- **THEN** el sistema SHALL loguear el error y enviar igual el correo, solo con el logo — un ícono faltante no bloquea el envío

#### Scenario: Envío genérico sin tipo (`Send`)
- **WHEN** se invoca `Client.Send` directamente (no `SendEmail`, sin `EmailType` conocido)
- **THEN** el correo SHALL incluir únicamente el logo como attachment, sin ícono de evento

### Requirement: Presentación visual consistente con la identidad de marca real
El sistema SHALL renderizar los templates de correo usando la paleta y escala de espaciado real de `paceron-frontend` (fondo blanco de card, texto `#111518`/`#40484c`, acento `#8cc63e`), sin depender de un componente específico de la app.

#### Scenario: Header sin banda de color sólido
- **WHEN** se renderiza cualquier template de correo
- **THEN** el header SHALL mostrar el logo sobre fondo blanco (no una banda de color sólido), apilado (ícono arriba, wordmark abajo)
