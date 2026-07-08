## ADDED Requirements

### Requirement: Login con email y password
El sistema SHALL proveer un endpoint `POST /api/v1/auth/login` que permita a los usuarios autenticarse con email y password.

#### Scenario: Login exitoso
- **WHEN** el usuario envía POST /api/v1/auth/login con email y password válidos de un usuario activo
- **THEN** el sistema retorna HTTP 200 con `{ access_token, refresh_token, expires_in: 3600 }`

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
El sistema SHALL generar un access token JWT firmado con HS256 que expire en 1 hora.

#### Scenario: Access token contiene claims correctos
- **WHEN** el sistema genera un access token para un login exitoso
- **THEN** el token SHALL contener sub (user ID), email, iat, exp

#### Scenario: Access token expira en 1 hora
- **WHEN** el sistema genera un access token
- **THEN** el claim exp SHALL ser iat + 3600 segundos

### Requirement: Generación de refresh token
El sistema SHALL generar un refresh token JWT firmado con HS256 que expire en 7 días.

#### Scenario: Refresh token contiene claims correctos
- **WHEN** el sistema genera un refresh token para un login exitoso
- **THEN** el token SHALL contener sub (user ID), iat, exp, type="refresh"

#### Scenario: Refresh token expira en 7 días
- **WHEN** el sistema genera un refresh token
- **THEN** el claim exp SHALL ser iat + 604800 segundos
