## Why

El login emite JWT desde `login-endpoint-con-jwt`, pero nada los valida: ningún endpoint verifica el access token, y el refresh token (otro JWT autocontenido) nunca se persiste ni se puede revocar. Cualquier `user_id` que mande el cliente se confía a ciegas. Esto bloquea el gap de frontend "búsqueda de usuarios logueado únicamente" y es la base necesaria para proteger el resto del backend (`feature/protect-all-endpoints`, rama siguiente).

## What Changes

- El refresh token deja de ser un JWT: pasa a ser un token opaco de alta entropía, persistido como hash SHA256 en una tabla nueva (`refresh_tokens`), para poder revocarlo y rotarlo.
- El access token gana `sid` (session ID, UUID) y `roles` (roles globales del usuario); ya no lleva datos mutables (`email`). Expira en 15 minutos (antes 1 hora) e incluye `iss`/`aud`.
- Nuevos endpoints `POST /api/v1/auth/refresh` (rota el refresh token: revoca el usado, emite un par nuevo con el mismo `session_id`) y `POST /api/v1/auth/logout` (revoca el refresh token, idempotente).
- `LoginResponse` se aplana: `{access_token, refresh_token, expires_in, user}` en vez de un objeto `authorization` anidado — cambio de contrato para el frontend.
- Nuevo `AuthMiddleware()` que valida el access token y deja la identidad en el contexto de Gin (`auth_user_id`, `auth_session_id`, `auth_roles`). **Se define en esta rama pero no se aplica a ninguna ruta todavía** — aplicarlo a los endpoints existentes es la rama siguiente (`feature/protect-all-endpoints`), para no mezclar la infraestructura de sesión con la migración de cada controller.
- `PASSWORD_PEPPER` (propuesto en un doc de referencia externo) queda explícitamente fuera de alcance — proyecto de tesis, no producto masivo, se prioriza simplicidad.

## Capabilities

### New Capabilities
- `session-management`: emisión, rotación y revocación de sesiones vía refresh tokens opacos persistidos; middleware de validación de access tokens.

### Modified Capabilities
- `user-login`: el contrato de `POST /api/v1/auth/login` cambia (claims del access token, forma de la respuesta, y ahora también persiste una sesión en vez de ser stateless).

## Impact

- **Nuevo modelo**: `dbs.RefreshToken` (+ AutoMigrate)
- **Nuevo DAO**: `RefreshTokenDao` (Create, FindActiveByHash, Revoke, RevokeBySessionID)
- **Reescritura**: `utils/jwt.go` (claims, `ParseAccessToken` de producción, `GenerateOpaqueToken`, `HashToken`)
- **Modificado**: `AuthService` (Login cambia, +Refresh, +Logout), `AuthController` (+Refresh, +Logout)
- **Nuevos domains**: `auth/RefreshRequest`, `auth/RefreshResponse`, `auth/LogoutRequest`, `auth/LogoutResponse`; `auth/LoginResponse` aplanado
- **Nueva config**: `JWT_ISSUER`, `JWT_AUDIENCE`, `ACCESS_TOKEN_DURATION`, `REFRESH_TOKEN_DURATION` (env con default)
- **Nuevo middleware**: `AuthMiddleware()` en `app/middleware.go` (sin aplicar a rutas aún)
- **Tests**: unit tests para DAO, jwt utils, service, controller y middleware — suite completa verde
- **Swagger**: regenerado
- **Frontend**: sin cambios en esta rama; el contrato nuevo se documenta para que el frontend lo consuma en un trabajo aparte, después de que también se mergee `feature/protect-all-endpoints`
