## 1. Config

- [x] 1.1 Agregar `JWTIssuer`, `JWTAudience`, `AccessTokenDuration`, `RefreshTokenDuration` a `config.go`, con default via env (`JWT_ISSUER`, `JWT_AUDIENCE`, `ACCESS_TOKEN_DURATION`, `REFRESH_TOKEN_DURATION`)

## 2. Modelo y DAO

- [x] 2.1 Crear `domains/dbs/refresh_token.go` con el modelo `RefreshToken` (PK `int64`, `session_id` UUID string, `token_hash` único, `expires_at`, `revoked_at`, `replaced_by`, `ip`, `user_agent`, `created_at`)
- [x] 2.2 Agregar `&dbs.RefreshToken{}` al `AutoMigrate` en `postgres.go`
- [x] 2.3 Crear `daos/refresh_token_dao.go`: `Create`, `FindActiveByHash`, `Revoke`, `RevokeBySessionID`
- [x] 2.4 Tests para el DAO

## 3. JWT Utils

- [x] 3.1 Reescribir `utils/jwt.go`: `AccessTokenClaims{SessionID, Roles, RegisteredClaims}` (sin email)
- [x] 3.2 `GenerateAccessToken(userID, sessionID, roles)` — 15 min, `iss`/`aud` desde config
- [x] 3.3 `ParseAccessToken(tokenString)` — wrapper de producción, valida firma/exp/iss/aud
- [x] 3.4 `GenerateOpaqueToken()` — `crypto/rand`, 32 bytes, base64 URL-safe
- [x] 3.5 `HashToken(token)` — SHA256 hex
- [x] 3.6 Eliminar `GenerateRefreshToken`/`RefreshTokenClaims` (JWT) — reemplazados por el token opaco
- [x] 3.7 Reescribir tests de jwt.go para el nuevo contrato

## 4. Domain structs

- [x] 4.1 Aplanar `domains/auth/login_response.go`: `{AccessToken, RefreshToken, ExpiresIn, User}`
- [x] 4.2 Crear `domains/auth/refresh_request.go` / `refresh_response.go`
- [x] 4.3 Crear `domains/auth/logout_request.go` / `logout_response.go`

## 5. Service Layer

- [x] 5.1 `AuthServiceInterface`: agregar `Refresh(ctx, refreshToken string)`, `Logout(ctx, refreshToken string)`
- [x] 5.2 `NewAuthService` gana `userRoleDao`, `roleDao`, `refreshTokenDao`
- [x] 5.3 `Login`: generar `session_id` (UUID), resolver roles globales del usuario, generar access token con esos claims, persistir refresh token opaco, devolver respuesta aplanada
- [x] 5.4 `Refresh`: hashear, buscar fila activa, validar usuario activo, rotar (crear nuevo antes de revocar el viejo, mismo `session_id`), devolver par nuevo
- [x] 5.5 `Logout`: hashear, revocar si existe (idempotente si no existe o ya estaba revocado)
- [x] 5.6 Tests de servicio: login persiste sesión/roles, refresh éxito, refresh con token inválido/expirado, refresh con usuario inactivo, logout éxito, logout idempotente con token desconocido

## 6. Controller y rutas

- [x] 6.1 `AuthController`: agregar `Refresh(c)`, `Logout(c)` con swagger godoc
- [x] 6.2 Registrar `POST /api/v1/auth/refresh` y `POST /api/v1/auth/logout` en `url_mappings.go` (sin `AuthMiddleware()`)
- [x] 6.3 Tests de controller: refresh éxito/token inválido/body inválido, logout éxito/body inválido

## 7. Middleware

- [x] 7.1 `AuthMiddleware()` en `app/middleware.go`: valida `Authorization: Bearer`, setea `auth_user_id`/`auth_session_id`/`auth_roles` en el contexto, 401 `token_expired` vs 401 `unauthorized`
- [x] 7.2 Tests: header faltante, header malformado, token inválido, token expirado, token válido (verifica los tres valores seteados en el contexto)
- [x] 7.3 **No aplicar a ninguna ruta todavía** — queda para `feature/protect-all-endpoints`

## 8. Wiring

- [x] 8.1 `app.go`: crear `refreshTokenDao`, pasar `userRoleDao`/`roleDao`/`refreshTokenDao` a `NewAuthService`

## 9. Verificación y docs

- [x] 9.1 `go build ./...` / `go vet ./...` / `go test ./...` verdes
- [x] 9.2 Regenerar Swagger
- [x] 9.3 Actualizar tabla de endpoints en `README.md`
- [ ] 9.4 Verificación manual: login → refresh (confirmar que el token viejo ya no sirve) → logout → confirmar que un refresh posterior con ese token falla
