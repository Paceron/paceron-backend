## Why

Actualmente la API permite registrar usuarios pero no autenticarlos. Se necesita un endpoint de login que valide credenciales contra la DB, verifique que el usuario esté activo, y genere tokens JWT para proteger los endpoints subsecuentes.

## What Changes

- Nuevo endpoint `POST /api/v1/auth/login` que recibe `email` y `password` en el body
- Validación de tipos de datos y que el usuario exista con estado `active`
- Validación de password contra el hash almacenado en DB (bcrypt)
- Generación de `access_token` (JWT) y `refresh_token` (JWT)
- Retorno de `{ access_token, refresh_token, expires_in }` con HTTP 200
- Retorno de HTTP 401 con mensaje de error si la autenticación falla
- Dependencia nueva: librería JWT para Go (golang-jwt/jwt)

## Capabilities

### New Capabilities
- `user-login`: Autenticación de usuarios con email/password, verificación de estado activo, emisión de tokens JWT (access + refresh) y refresh de tokens.

### Modified Capabilities
- (ninguna)

## Impact

- **Nuevo controller**: `authController.Login` (o un controller de auth ya existe)
- **Nuevo service**: `AuthService.Login` (extender interface existente)
- **Nuevo DAO method**: buscar usuario por email (ya existe `FindByEmail`)
- **Nueva dependencia**: `github.com/golang-jwt/jwt/v5`
- **Nuevos domains**: `auth/LoginRequest`, `auth/LoginResponse`
- **Tests**: unit tests para controller, service; integration si aplica
- **Swagger**: actualizar docs
