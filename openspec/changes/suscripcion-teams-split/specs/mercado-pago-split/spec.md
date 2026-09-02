## ADDED Requirements

### Requirement: Conexión OAuth del entrenador (seller_connections)

El sistema SHALL permitir que un usuario con rol `entrenador` conecte su cuenta de Mercado Pago vía OAuth mp-connect para poder cobrar las mensualidades de su equipo con split. La conexión SHALL persistirse en `seller_connections` (`user_id` único, `mp_user_id`, `access_token` cifrado en repositorio, estado `authorized`/`deauthorized`, fechas). El access_token SHALL NO loguearse ni exponerse en ninguna respuesta de API.

#### Scenario: Entrenador sin conectar intenta conectar
- **WHEN** un entrenador sin conexión pide la URL de autorización (`GET /api/v1/mercadopago/connect`)
- **THEN** el sistema devuelve la URL de mercado pago con un `state` (CSRF) asociado al usuario

#### Scenario: Callback de OAuth exitoso
- **WHEN** el entrenador vuelve del callback con `code` y un `state` válido
- **THEN** el sistema intercambia el `code` por un token mediante la cuenta de la app, persiste la conexión como `authorized` y devuelve éxito

#### Scenario: Callback con state inválido
- **WHEN** el `state` del callback no coincide con el emitido para el usuario
- **THEN** el sistema rechaza la operación

#### Scenario: Estado de conexión
- **WHEN** se consulta `GET /api/v1/mercadopago/connect/status`
- **THEN** el sistema devuelve `connected` y `account_status` (`authorized`/`deauthorized`) del entrenador logueado

#### Scenario: Desautorización desde Mercado Pago
- **WHEN** el webhook de Mercado Pago notifica que la conexión fue desautorizada
- **THEN** el sistema actualiza `seller_connections` a `deauthorized`

### Requirement: Comisión de Paceron configurable (platform_settings)

El sistema SHALL exponer una configuración global `marketplace_fee_percent` (porcentaje de comisión que Paceron retiene del split) en `platform_settings` (tabla genérica key-value). La lectura (`GET /api/v1/platform-settings/marketplace-fee`) SHALL estar disponible para cualquier usuario autenticado; la escritura (`PUT`) SHALL ser exclusiva de usuarios con el rol de sistema `admin`.

El access_token y el refresh_token de `seller_connections` SHALL persistirse cifrados en repositorio (AES-GCM, clave desde config) y SHALL NO loguearse ni exponerse en ninguna respuesta de API.

#### Scenario: Consultar la comisión actual
- **WHEN** un usuario autenticado consulta la comisión
- **THEN** devuelve `marketplace_fee_percent` (default 5.0) y su `updated_at`

#### Scenario: Actualizar la comisión como owner
- **WHEN** el owner actualiza la comisión a 7.5
- **THEN** el sistema persiste y devuelve el nuevo valor

#### Scenario: Actualizar la comisión sin ser owner
- **WHEN** un usuario no owner intenta actualizar
- **THEN** el sistema rechaza con error de autorización

### Requirement: Procesamiento de pagos con split

Al crear una preferencia o procesar un pago con `concept = team_subscription`, el sistema SHALL resolver la cuota por `installment_id`, identificar el equipo (instalación → `team_id`) y a su dueño (`teams.owner_id`), cargar su conexión `seller_connections` (`authorized`), y crear la preferencia/pago en Mercado Pago con el `access_token` del dueño aplicando `marketplace_fee` calculado como `redondeo(amount * marketplace_fee_percent / 100)`. El registro `payments` SHALL guardar `concept = team_subscription`, `seller_user_id = team.owner_id` y `marketplace_fee`. Sin conexión autorizada del dueño, el sistema SHALL rechazar la operación.

#### Scenario: Pagar una cuota de equipo con split
- **WHEN** se solicita preferencia para una cuota de equipo y el dueño está conectado
- **THEN** el sistema usa el token del dueño y aplica `marketplace_fee`
- **AND** `payments` registra `concept = team_subscription`, `seller_user_id` y `marketplace_fee`

#### Scenario: Dueño del equipo sin conexión OAuth
- **WHEN** se solicita preferencia para una cuota de equipo y el dueño no está conectado (o `deauthorized`)
- **THEN** el sistema rechaza la operación indicando que el entrenador debe conectar su cuenta

### Requirement: El webhook reconcilia cuotas de equipo

Cuando el webhook de Mercado Pago confirma un pago `approved` vinculado a una cuota de equipo, el sistema SHALL marcarla `paid` idempotentemente, incrementar `paid_installments` del `team_user` y, si era la cuota #1, activar la membresía; si era una cuota posterior, generar la siguiente (reglas de `team-subscriptions`). El split ya fue aplicado en Mercado Pago al crear el pago; el webhook no recalcula comisiones.

#### Scenario: Confirmación de una cuota recurrente de equipo
- **WHEN** el webhook confirma una cuota con `installment_number > 1` de un miembro activo
- **THEN** la cuota pasa a `paid` y se incrementa `paid_installments`
- **AND** se genera la cuota siguiente