## Context

Hoy la app no valida el JWT en ningún endpoint — se genera en login (`utils.GenerateAccessToken`/`GenerateRefreshToken`) pero no hay middleware ni código en ningún controller que lo lea o verifique; todo endpoint identifica al usuario por el `:id` de la URL. El refresh token tampoco se consume en ningún lado. Confirmado explorando `cmd/api/app/middleware.go` completo y grepeando "Authorization"/JWT-parsing en todo `cmd/api` — no hay hits fuera de tests.

Ya existe precedente directo para "verificar contraseña actual antes de un cambio sensible": `UserController.Update`/`user_service.go` piden header `X-Current-Password` para permitir cambio de email, verificado con `bcrypt.CompareHashAndPassword`.

## Goals / Non-Goals

**Goals:**
- Endpoint de cambio de contraseña autenticado, mismo nivel de rigor que el resto de la app (identificación por `:id`, verificación de contraseña actual).
- Dejar preparado (no enforced) el dato para futura invalidación de sesiones.

**Non-Goals:**
- Construir el primer middleware de auth/JWT de la app — cambio de arquitectura grande, se aborda aparte cuando se decida.
- `/refresh`, `/logout` — quedan para una feature futura (`/logout` es la siguiente, explícitamente).
- Notificar por mail el cambio.

## Decisions

### 1. Endpoint `PATCH /api/v1/users/:id/password`, no parte de `Update`
**Por qué**: mismo criterio que `PATCH /api/v1/users/:id/status` — un campo sensible con su propia validación (verificación de contraseña actual, fortaleza, confirmación) se separa del `PUT` genérico de actualización de perfil, en vez de sobrecargar `UserUpdateRequest` con 3 campos opcionales de contraseña.

### 2. Validación de fortaleza/confirmación en el controller, no en el service
**Por qué**: mismo patrón ya usado en `Register` (`ValidatePassword` se llama desde `auth_controller.go`) y en `ResetPassword` (mismatch/fortaleza en `password_reset_controller.go`). El service (`ChangePassword`) se ocupa solo de la verificación de seguridad (contraseña actual correcta) y la persistencia.

### 3. Rechazar si la nueva contraseña es igual a la actual
**Por qué**: buena práctica barata — un "cambio" que deja la misma contraseña no tiene sentido y probablemente sea un error del usuario. Se verifica con `bcrypt.CompareHashAndPassword(oldHash, newPlaintext) == nil` (si compara OK, es la misma) antes de hashear la nueva.
**Alternativa descartada**: no validar esto — se descarta porque el costo de agregarlo es una sola comparación bcrypt, y evita un caso confuso sin costo real.

### 4. `password_changed_at` se agrega ahora, sin middleware que la use
**Por qué**: es aditivo y barato (una columna nullable, AutoMigrate la agrega sola). Preparar el dato ahora evita tener que hacer un backfill después cuando se decida construir el enforcement real (JWT claim + middleware que compare contra este campo) — ese enforcement sería el primer guard de auth de toda la app, cambio de arquitectura que excede el alcance de esta feature puntual.
**Alternativa descartada**: no agregar la columna hasta que se construya el middleware — se descarta porque implicaría un backfill/migración de datos históricos imposible (no se puede reconstruir cuándo cambió la contraseña de un usuario existente después del hecho); agregarla ahora, aunque no se use todavía, es la única forma de que el dato exista desde este momento en adelante.

### 5. `password_reset_service.ResetPassword` también setea `password_changed_at`
**Por qué**: es el otro camino real de cambio de contraseña (forgot/reset por OTP). Si solo el `ChangePassword` nuevo setea el campo, el dato queda incompleto/engañoso — un usuario que reseteó por mail parecería "nunca cambió su contraseña". Se agrega la línea directo en el `ResetPassword` ya mergeado (PR #11 cerrada, sin problema de tocar código en review activo).

## Risks / Trade-offs

- Sin enforcement real, un JWT emitido antes de un cambio de contraseña sigue siendo válido hasta su expiración natural (1h para access, 7d para refresh aunque no se usa) — riesgo de seguridad ya preexistente en toda la app, no introducido ni empeorado por este cambio, y explícitamente fuera de alcance resolverlo acá.
