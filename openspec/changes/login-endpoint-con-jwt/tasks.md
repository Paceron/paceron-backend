## 1. Setup

- [x] 1.1 Agregar dependencia `github.com/golang-jwt/jwt/v5` al go.mod
- [x] 1.2 Agregar `JWT_SECRET` a la configuración y levantar de variable de entorno

## 2. JWT Utils

- [x] 2.1 Crear `utils/jwt.go` con `GenerateAccessToken(userID int64, email string) (string, error)` (HS256, 1h exp)
- [x] 2.2 Crear `utils/jwt.go` con `GenerateRefreshToken(userID int64) (string, error)` (HS256, 7d exp, claim type="refresh")

## 3. Domain Structs

- [x] 3.1 Crear `domains/auth/login_request.go` con `LoginRequest { Email, Password string binding:"required" }`
- [x] 3.2 Crear `domains/auth/login_response.go` con `LoginResponse { AccessToken, RefreshToken string, ExpiresIn int }`

## 4. Service Layer

- [x] 4.1 Agregar método `Login(ctx, email, password string) (*auth.LoginResponse, error)` a `AuthServiceInterface`
- [x] 4.2 Implementar `Login` en `authService`: buscar usuario por email, verificar status active, comparar bcrypt, generar tokens, retornar response

## 5. Controller Layer

- [x] 5.1 Agregar método `Login(c *gin.Context)` a `AuthController`
- [x] 5.2 Implementar `Login` en controller: bindear body, validar, llamar service, manejar errores 400/401/500, retornar 200 con tokens

## 6. Routes

- [x] 6.1 Registrar `POST /api/v1/auth/login` en url_mappings.go

## 7. Swagger

- [x] 7.1 Agregar anotaciones swagger en el controller para el endpoint login
- [x] 7.2 Regenerar docs con `swag init`

## 8. Tests

- [x] 8.1 Agregar tests unitarios en `controllers/auth_controller_test.go` para login (éxito, email inválido, password incorrecta, usuario inactivo, body inválido)
- [x] 8.2 Agregar tests unitarios en `services/auth_service_test.go` para login (éxito, email no encontrado, password incorrecta, usuario inactivo)
- [x] 8.3 Agregar tests para `utils/jwt.go` (generación y claims de ambos tokens)
- [x] 8.4 Verificar que todos los tests pasen y `go vet` limpio
