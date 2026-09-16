## MODIFIED Requirements

### Requirement: Conexión OAuth del entrenador (seller_connections)

El sistema SHALL permitir que un usuario con rol `entrenador` conecte su cuenta de Mercado Pago vía OAuth mp-connect para poder cobrar las mensualidades de su equipo con split. La conexión SHALL persistirse en `seller_connections` (`user_id` único, `mp_user_id`, `access_token` cifrado en repositorio, estado `authorized`/`deauthorized`, fechas). El access_token SHALL NO loguearse ni exponerse en ninguna respuesta de API.

El callback de OAuth SHALL terminar en un redirect HTTP al frontend en vez de responder JSON, porque es una navegación del usuario y no una llamada de API. El destino del redirect SHALL resolverse contra configuración del servidor a partir del `target` codificado en el `state`, y el `code` de autorización SHALL NO incluirse en la URL de retorno.

#### Scenario: Entrenador sin conectar intenta conectar
- **WHEN** un entrenador sin conexión pide la URL de autorización (`GET /api/v1/mercadopago/connect`)
- **THEN** el sistema devuelve la URL de mercado pago con un `state` (CSRF) asociado al usuario

#### Scenario: Se elige el destino de retorno al pedir la URL de autorización
- **WHEN** se invoca `GET /api/v1/mercadopago/connect?platform=app`
- **THEN** el `state` emitido SHALL codificar el target `app`, de modo que el callback redirija al deep link de la aplicación nativa

#### Scenario: Valor de plataforma desconocido o ausente
- **WHEN** se invoca `GET /api/v1/mercadopago/connect` sin `platform`, o con un valor distinto de `web`/`app`
- **THEN** el `state` SHALL codificar el target `web`

#### Scenario: Callback de OAuth exitoso
- **WHEN** el entrenador vuelve del callback con `code` y un `state` válido
- **THEN** el sistema intercambia el `code` por un token mediante la cuenta de la app, persiste la conexión como `authorized` y SHALL responder `302` hacia la URL de retorno correspondiente al target del `state`, con `status=success` en la query

#### Scenario: Callback con state inválido
- **WHEN** el `state` del callback no coincide con el emitido para el usuario
- **THEN** el sistema rechaza la operación y SHALL responder `302` hacia la URL de retorno con `status=error` y un `reason` que identifique la causa, sin incluir el texto crudo del error

#### Scenario: State emitido antes de la incorporación del target
- **WHEN** llega un callback cuyo `state` tiene solo dos segmentos (`<userID>-<timestamp>`)
- **THEN** el sistema SHALL interpretarlo como target `web` y procesarlo normalmente

#### Scenario: URLs de retorno no configuradas
- **WHEN** llega un callback y el entorno no tiene configurada la URL de retorno del target correspondiente
- **THEN** el sistema SHALL responder el JSON de resultado como antes, en vez de redirigir a un destino vacío

#### Scenario: Estado de conexión
- **WHEN** se consulta `GET /api/v1/mercadopago/connect/status`
- **THEN** el sistema devuelve `connected` y `account_status` (`authorized`/`deauthorized`) del entrenador logueado

#### Scenario: Desautorización desde Mercado Pago
- **WHEN** el webhook de Mercado Pago notifica que la conexión fue desautorizada
- **THEN** el sistema actualiza `seller_connections` a `deauthorized`
