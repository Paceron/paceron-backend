## Context

`login-endpoint-con-jwt` dejó login funcionando pero explícitamente fuera de alcance: "Refresh de tokens", "Middleware de autenticación", "Invalidez de tokens (logout)". Este cambio cierra esos tres puntos, adoptando (adaptado a este dominio y a las convenciones del repo) un diseño de sesiones que el usuario compartió como referencia externa.

## Goals / Non-Goals

**Goals:**
- Refresh tokens revocables y rotables, persistidos solo como hash.
- Access tokens de vida corta (15 min) sin datos mutables del usuario.
- `AuthMiddleware()` reusable, mismo patrón que `CORSMiddleware()`.
- `POST /auth/refresh` y `POST /auth/logout` funcionando end to end.

**Non-Goals (quedan para `feature/protect-all-endpoints`):**
- Aplicar `AuthMiddleware()` a las rutas existentes.
- Migrar controllers/services para usar `auth_user_id` del contexto en vez del `user_id` que manda el cliente.
- Autorización (self-vs-delegado, entrenador-del-equipo, etc.) — sigue viviendo en la capa de servicio, no en un middleware genérico (ABAC necesita datos del recurso ya cargado, no solo la identidad).
- `PASSWORD_PEPPER` — descartado, complejidad innecesaria para este proyecto.

## Decisions

### 1. Refresh token: opaco, no JWT
**Por qué**: un JWT de refresh es autocontenido y no se puede revocar sin una lista de bloqueo. Un token opaco de alta entropía (`crypto/rand`, 32 bytes) obliga a mirar la base de datos en cada refresh, que es exactamente lo que se necesita para poder rotar y detectar reuso/robo.
**Alternativa descartada**: seguir con JWT de refresh + tabla de revocación por `jti` — más complejo sin ganar nada, ya que el lookup a DB es inevitable de todos modos.

### 2. `RefreshToken.ID`: `int64` autoincremental, no UUID
**Por qué**: consistencia con el resto de las tablas del repo (todas usan PK `int64`). El UUID se reserva para `session_id`, que es el identificador que de verdad necesita ser correlacionable entre las filas de una misma cadena de rotación (no la PK de cada fila individual).

### 3. Claims del access token: `sid` + `roles`, sin datos mutables
**Por qué**: `email`/nombre pueden cambiar durante la vida del token (15 min es corto, pero el patrón se mantiene por consistencia); el rol si cambia, el próximo refresh (o el próximo login) ya lo refleja. `sid` (session ID) es lo que permite, a futuro, invalidar todos los tokens de una sesión sin tocar las demás sesiones del mismo usuario.
**Roles usados**: el sistema de roles global existente (`user_role`/`role`), no `TeamUser.RoleInTeam` (que es por equipo, no global) — son conceptos distintos en este dominio.

### 4. Rotación de refresh: crear-nuevo-antes-de-revocar-el-viejo
**Por qué**: mismo patrón de recuperación ante fallo parcial ya usado en otros flujos de este backend (ej. aceptar invitación) — si falla a mitad de camino, es preferible terminar con dos tokens válidos temporalmente a terminar con cero tokens válidos y a la sesión bloqueada.

### 5. Middleware definido pero no aplicado en esta rama
**Por qué**: aplicar `AuthMiddleware()` implica migrar cada controller a leer identidad del contexto en vez del `user_id` que manda el cliente — son ~15 controllers con casos de autorización distintos (self-only, self-o-delegado, solo-entrenador). Mezclarlo con la infraestructura de sesión en un solo cambio lo vuelve inrevisable. Se secuencia en dos ramas: esta (infraestructura) y `feature/protect-all-endpoints` (migración).

### 6. Contrato de error del middleware: `apierror.APIError` existente
**Por qué**: consistencia con el resto de la API — se usa el mismo envelope `{status_code, code, message}` que ya devuelven todos los demás errores, en vez de inventar una forma nueva. `code: "token_expired"` vs `code: "unauthorized"` para que el frontend distinga "hacé refresh" de "logueate de nuevo".

### 7. `PASSWORD_PEPPER` descartado
**Por qué**: decisión explícita del usuario — mantener la implementación simple, sin complejidad extra no pedida, dado que es un proyecto de tesis y no un producto para una base masiva de usuarios.

## Risks / Trade-offs

- **Sesiones huérfanas**: filas de `refresh_tokens` vencidas o revocadas se acumulan sin limpieza automática (no hay job de purga). Aceptado por ahora — volumen de usuarios bajo, se puede agregar un `DELETE ... WHERE expires_at < NOW() - INTERVAL` más adelante si hace falta.
- **Ventana de 15 minutos sin roles actualizados**: si a un usuario le cambian los roles a mitad de una sesión, el cambio no se refleja hasta el próximo refresh o login. Aceptado — la alternativa (roles fuera del token, consultados en cada request) agrega una query por request sin necesidad real hoy.
- **Middleware sin aplicar todavía**: durante el período entre esta rama y `feature/protect-all-endpoints`, el backend sigue sin protección real pese a tener toda la infraestructura lista. Riesgo aceptado porque ambas ramas se mergean en secuencia inmediata, sin trabajo no relacionado en el medio.
