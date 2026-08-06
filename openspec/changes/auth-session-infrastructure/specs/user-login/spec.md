## MODIFIED Requirements

### Requirement: Login con email y password
El sistema SHALL proveer un endpoint `POST /api/v1/auth/login` que permita a los usuarios autenticarse con email y password, iniciando una sesión persistida.

#### Scenario: Login exitoso
- **WHEN** el usuario envía POST /api/v1/auth/login con email y password válidos de un usuario activo
- **THEN** el sistema genera una sesión nueva (`session_id`), persiste un refresh token asociado, y retorna HTTP 200 con `{ access_token, refresh_token, expires_in, user }`

#### Scenario: Email inválido
- **WHEN** el usuario envía POST /api/v1/auth/login con un email que no existe en la DB
- **THEN** el sistema retorna HTTP 401 con mensaje "No se pudo autenticar"

#### Scenario: Password incorrecta
- **WHEN** el usuario envía POST /api/v1/auth/login con un email existente pero password incorrecta
- **THEN** el sistema retorna HTTP 401 con mensaje "No se pudo autenticar"

#### Scenario: Usuario no activo
- **WHEN** el usuario envía POST /api/v1/auth/login con credenciales válidas pero el usuario no está en estado "active"
- **THEN** el sistema retorna HTTP 401 con mensaje "No se pudo autenticar"

#### Scenario: Campos requeridos faltantes
- **WHEN** el usuario envía POST /api/v1/auth/login sin email o sin password
- **THEN** el sistema retorna HTTP 400 con mensaje de validación

#### Scenario: Body inválido
- **WHEN** el usuario envía POST /api/v1/auth/login con un body mal formado (no JSON)
- **THEN** el sistema retorna HTTP 400 con mensaje "Cuerpo de solicitud inválido"

### Requirement: Generación de access token
El sistema SHALL generar un access token JWT firmado con HS256 que expire en 15 minutos, atado a una sesión y sin datos mutables del usuario.

#### Scenario: Access token contiene claims correctos
- **WHEN** el sistema genera un access token para un login o refresh exitoso
- **THEN** el token SHALL contener `sub` (user ID), `sid` (session ID), `roles` (roles globales del usuario), `iat`, `exp`, `iss`, `aud`

#### Scenario: Access token expira en 15 minutos
- **WHEN** el sistema genera un access token
- **THEN** el claim `exp` SHALL ser `iat` + 900 segundos (configurable vía `ACCESS_TOKEN_DURATION`)

### Requirement: Generación de refresh token
El sistema SHALL generar, en cada login, un refresh token opaco (no JWT) persistido como hash, que expire por defecto en 30 días.

#### Scenario: Refresh token no es un JWT
- **WHEN** el sistema genera un refresh token para un login exitoso
- **THEN** el token SHALL ser un secreto aleatorio de alta entropía sin estructura de claims propia — su significado solo existe buscándolo por hash en `refresh_tokens`

#### Scenario: Refresh token expira en 30 días por defecto
- **WHEN** el sistema genera un refresh token
- **THEN** `expires_at` SHALL ser `created_at` + `REFRESH_TOKEN_DURATION` (default 30 días)
