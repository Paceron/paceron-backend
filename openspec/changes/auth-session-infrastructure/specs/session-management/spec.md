## ADDED Requirements

### Requirement: Refresh tokens opacos y persistidos
El sistema SHALL emitir refresh tokens como secretos opacos de alta entropía (no JWT), y SHALL persistir únicamente su hash SHA256, nunca el token en texto plano.

#### Scenario: Refresh token generado en login
- **WHEN** un usuario hace login exitoso
- **THEN** el sistema genera un token aleatorio de 32 bytes, calcula su hash SHA256, y persiste una fila en `refresh_tokens` con ese hash, `user_id`, `session_id` nuevo (UUID) y `expires_at`

### Requirement: Rotación de refresh tokens
El sistema SHALL proveer `POST /api/v1/auth/refresh` que, dado un refresh token vigente, revoca ese token y emite un par access+refresh nuevo atado a la misma sesión.

#### Scenario: Refresh exitoso
- **WHEN** se envía `POST /api/v1/auth/refresh` con un `refresh_token` activo (no vencido, no revocado)
- **THEN** el sistema revoca la fila usada (`revoked_at`, `replaced_by` apuntando a la fila nueva), crea una fila nueva con el mismo `session_id`, y retorna HTTP 200 con `{access_token, refresh_token, expires_in}`

#### Scenario: Refresh token inválido, vencido o ya revocado
- **WHEN** se envía `POST /api/v1/auth/refresh` con un `refresh_token` que no existe, está vencido, o ya fue revocado
- **THEN** el sistema retorna HTTP 401 con un mensaje genérico (no distingue el motivo, para no filtrar información)

#### Scenario: Usuario inactivo
- **WHEN** se envía `POST /api/v1/auth/refresh` con un token válido pero el usuario asociado ya no está `active`
- **THEN** el sistema retorna HTTP 401

### Requirement: Logout revoca la sesión
El sistema SHALL proveer `POST /api/v1/auth/logout` que revoca el refresh token indicado, de forma idempotente.

#### Scenario: Logout exitoso
- **WHEN** se envía `POST /api/v1/auth/logout` con un `refresh_token` activo
- **THEN** el sistema marca esa fila como revocada y retorna HTTP 200

#### Scenario: Logout con token ya revocado o inexistente
- **WHEN** se envía `POST /api/v1/auth/logout` con un `refresh_token` que no existe o ya estaba revocado
- **THEN** el sistema retorna HTTP 200 igualmente (idempotente, no filtra si el token existió alguna vez)

### Requirement: Middleware de validación de access token
El sistema SHALL proveer `AuthMiddleware()`, que valida el access token del header `Authorization: Bearer <token>` y deja la identidad autenticada disponible para el resto del request.

#### Scenario: Header ausente o malformado
- **WHEN** una request llega sin header `Authorization`, o sin el prefijo `Bearer `
- **THEN** el middleware aborta con HTTP 401 y `code: "unauthorized"`

#### Scenario: Token inválido
- **WHEN** el token no verifica (firma inválida, `iss`/`aud` incorrectos, formato inválido)
- **THEN** el middleware aborta con HTTP 401 y `code: "unauthorized"`

#### Scenario: Token expirado
- **WHEN** el token es válido pero su `exp` ya pasó
- **THEN** el middleware aborta con HTTP 401 y `code: "token_expired"` (distinto del resto de fallos, para que el cliente sepa que debe hacer refresh)

#### Scenario: Token válido
- **WHEN** el token es válido y no expiró
- **THEN** el middleware setea `auth_user_id`, `auth_session_id` y `auth_roles` en el contexto de la request y continúa la cadena normalmente
