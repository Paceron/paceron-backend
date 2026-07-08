## Context

La API actual tiene registro de usuarios pero no autenticación. Se debe implementar un endpoint de login que genere tokens JWT. Los usuarios existentes ya tienen password hasheado con bcrypt y campo `status`. Se necesita agregar una librería JWT para generar los tokens.

## Goals / Non-Goals

**Goals:**
- Endpoint `POST /api/v1/auth/login` que acepte `email` y `password` en el body
- Validación de que el usuario exista, esté en estado `active` y la password coincida
- Generación de `access_token` y `refresh_token` JWT firmados con HMAC-SHA256
- Retorno de `{ access_token, refresh_token, expires_in: 3600 }` con HTTP 200
- Retorno de HTTP 401 con mensaje de error si falla autenticación
- Logging de intentos fallidos y exitosos via customlogger

**Non-Goals:**
- Refresh de tokens (se hará en un cambio futuro)
- Roles/permisos
- Middleware de autenticación para rutas existentes (se hará después)
- Invalidez de tokens (logout)

## Decisions

### 1. Librería JWT: golang-jwt/jwt v5
- **Por qué**: Es la librería JWT más adoptada en Go, mantenida activamente, soporta HMAC, RSA, ECDSA. Versión 5 es la actual estable.
- **Alternativa**: `lestrrat-go/jwx` — más completa pero más compleja para lo que necesitamos.

### 2. Algoritmo de firma: HMAC-SHA256 (HS256)
- **Por qué**: Simple, rápido, suficiente para backend monolítico. La clave secreta se configura vía variable de entorno `JWT_SECRET`.
- **Alternativa**: RSA256 — innecesario sin múltiples servicios.

### 3. Claims del access_token
- **sub**: user ID
- **email**: user email
- **iat**: issued at
- **exp**: expiration (1 hora desde ahora)

### 4. Claims del refresh_token
- **sub**: user ID
- **iat**: issued at
- **exp**: expiration (7 días desde ahora)
- **type**: "refresh"

### 5. Ubicación del nuevo código
- **Domains**: `domains/auth/login_request.go`, `domains/auth/login_response.go`
- **Service**: método `Login` en `AuthServiceInterface`/`authService`
- **Controller**: método `Login` en `AuthController`
- **JWT utils**: `utils/jwt.go` con funciones `GenerateAccessToken`, `GenerateRefreshToken`, `ValidateToken`
- **Config**: agregar `JWTSecret` a la configuración

### 6. Flujo de login
1. Controller recibe body → bindea a `LoginRequest`
2. Valida campos requeridos
3. Service busca usuario por email via DAO
4. Verifica que `status == "active"`
5. Compara password con bcrypt
6. Genera access_token (1h) y refresh_token (7d)
7. Retorna `LoginResponse`

## Risks / Trade-offs

- **Clave JWT en config** → Mitigación: documentar que debe ser un secreto fuerte (32+ chars) y rotarse periódicamente, nunca comitearse
- **Refresh tokens sin almacenar** → Decisión consciente: como no hay invalidez de tokens, no necesitamos almacenarlos. Se agregará en futuro si se requiere logout
- **HMAC simétrico** → Si en futuro hay múltiples servicios, migrar a RSA
