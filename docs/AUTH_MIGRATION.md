# Migración de auth: qué cambia para el frontend

Documento de referencia para el equipo de frontend, generado al cerrar `feature/auth-session-infrastructure` + `feature/protect-all-endpoints`. Resume el contrato nuevo — no reemplaza el detalle endpoint por endpoint de Swagger (`/swagger/index.html`).

## 1. Contrato de sesión nuevo

### Login — `POST /api/v1/auth/login`

Antes devolvía `{ authorization: { access_token, refresh_token, expires_in }, user }`. Ahora la respuesta está aplanada:

```json
{
  "access_token": "eyJ...",
  "refresh_token": "aXvd...",
  "expires_in": 900,
  "user": { "user_id": 1, "name": "...", "email": "...", ... }
}
```

- `access_token`: JWT, expira en **15 minutos** (antes 1 hora). No confiar en su duración en el cliente sin manejar el refresh.
- `refresh_token`: ya **no es un JWT** — es un string opaco. No intentar decodificarlo ni leer nada de él en el cliente, es solo un secreto que se manda de vuelta al backend.

### Refresh — `POST /api/v1/auth/refresh` (nuevo)

```json
// request
{ "refresh_token": "aXvd..." }

// response 200
{ "access_token": "eyJ...", "refresh_token": "nuevo-token...", "expires_in": 900 }

// response 401 (token inválido, vencido, revocado, o ya usado)
{ "status_code": 401, "code": "Unauthorized", "message": "refresh token inválido o expirado" }
```

**Importante — rotación**: cada llamada a `/refresh` invalida el `refresh_token` usado y devuelve uno nuevo. El cliente debe **reemplazar** el refresh token guardado por el que viene en la respuesta, no reusar el viejo. Si se intenta reusar un refresh token ya rotado, el backend lo rechaza (esto también sirve como detección de robo de token — si el cliente legítimo recibe 401 en refresh sin haber cerrado sesión antes, es señal de que el token fue usado por otro lado).

### Logout — `POST /api/v1/auth/logout` (nuevo)

```json
// request
{ "refresh_token": "aXvd..." }

// response 200 (siempre, sea el token válido, ya revocado, o inexistente — idempotente)
{ "message": "Sesión cerrada correctamente" }
```

El access token emitido antes del logout sigue siendo técnicamente válido hasta que expire naturalmente (máximo 15 min) — logout no lo invalida, invalida el refresh token para que no se pueda renovar la sesión.

## 2. Qué endpoints ahora exigen `Authorization: Bearer <access_token>`

**Regla general: todos, excepto los listados abajo.** Ver la tabla completa en [`README.md`](../README.md#endpoints) (marcados con 🔓).

Rutas públicas (sin header):
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh` (usa el refresh token como credencial propia)
- `POST /api/v1/auth/logout` (idem)
- `GET /api/v1/auth/user?id=&email=` (lookup público, decisión explícita)
- `POST /api/v1/auth/forgot-password`
- `POST /api/v1/auth/reset-password`
- `GET /user/:user_id`, `POST /user` (legacy, deprecados, no tocar)
- `GET /example/weather`, `GET /user/:user_id/weather` (demo)
- `GET /swagger`

Si el cliente llama a cualquier otro endpoint sin el header, o con un access token vencido/inválido, el backend responde `401`:

```json
// header ausente o formato inválido, o token con firma/issuer/audience inválidos
{ "status_code": 401, "code": "unauthorized", "message": "..." }

// token válido pero vencido — distinto código a propósito
{ "status_code": 401, "code": "token_expired", "message": "el access token expiró" }
```

**Recomendación de manejo en el cliente**: en un 401 con `code: "token_expired"`, intentar `POST /api/v1/auth/refresh` una vez con el refresh token guardado, y reintentar el request original con el access token nuevo. En cualquier otro `401` (`code: "unauthorized"`, o refresh también falla), redirigir a login — la sesión no es recuperable.

## 3. Parámetros que desaparecieron de las URLs (ahora se resuelven del token)

Estos endpoints antes recibían la identidad del usuario como query param o campo del body, controlado por el cliente. Ahora la identidad sale del access token — **el cliente ya no debe mandar estos parámetros, y si los manda se ignoran**:

| Endpoint | Antes | Ahora |
|---|---|---|
| `DELETE /api/v1/teams/:id` | `?user_id=` | resuelto del token |
| `DELETE /api/v1/groups/:id` | `?user_id=` | resuelto del token |
| `GET /api/v1/groups?team_id=` | `&user_id=` (requerido junto con team_id) | resuelto del token |
| `GET /api/v1/invitations` | `?user_id=` (requerido) | resuelto del token, ya no requiere query |
| `GET /api/v1/invitations/:id` | `?user_id=` (requerido) | resuelto del token |
| `POST /api/v1/invitations/:id/accept` | body `{"user_id": ...}` | resuelto del token, **sin body** |
| `POST /api/v1/invitations/:id/reject` | body `{"user_id": ...}` | resuelto del token, **sin body** |
| `POST /api/v1/teams` | body `{"owner_id": ...}` | resuelto del token — el owner siempre es quien crea el equipo |

## 4. Nuevas reglas de autorización (antes no existían)

Antes casi ninguna operación de escritura sobre equipos/grupos verificaba quién llamaba — se confiaba en lo que mandara el cliente. Ahora:

- **Solo entrenador del equipo**: crear/actualizar/borrar equipo, actualizar dirección, agregar usuarios al equipo, crear/actualizar/borrar grupo, agregar usuarios a un grupo, invitar corredores, **listar las invitaciones pendientes de un equipo** (`GET /api/v1/teams/:id/invitations` — antes cualquier usuario logueado podía ver nombres/emails de invitados de cualquier equipo con solo saber el `team_id`). Si un corredor (o alguien sin membresía) intenta estas acciones, `403 Forbidden`.
- **Self o entrenador delegado**: sacarse a sí mismo de un equipo/grupo, o que el entrenador saque a otro. Cualquier otro intento (ej. un corredor tratando de sacar a otro corredor) → `403 Forbidden`.
- **Cualquier miembro del equipo (no exclusivo del entrenador)**: `GET /api/v1/teams/:id/users` y `GET /api/v1/groups/:id/users` (listar el roster) — antes cualquier usuario logueado podía ver el roster de cualquier equipo/grupo sin ser miembro, solo adivinando el ID. Ahora requiere pertenecer al equipo (cualquier rol).
- **Self-only**: actualizar el propio usuario, cambiar el propio estado, cambiar la propia contraseña, asignarse/quitarse un rol (`POST/DELETE /api/v1/users/:id/roles`) — antes sin ningún chequeo, cualquier logueado podía asignar o sacar roles de cualquier otra cuenta. Intentar modificar el usuario/roles de otro (aun autenticado) → `403 Forbidden`.

Nuevo `code` en las respuestas de error para estos casos: `"Forbidden"`, `status_code: 403`.

## 5. Activación/desactivación del rol entrenador (nuevo)

`POST /api/v1/users/:id/roles` sigue existiendo genérico (self-only) para casos como el rol base `corredor`, pero volverse entrenador tiene reglas propias — es la capacidad que desbloquea crear equipos y todos los chequeos "solo entrenador" de la sección 4, así que amerita su propio endpoint en vez de un `role_id` genérico a ciegas:

- `POST /api/v1/users/:id/trainer-role` — activa el rol. Self-only. Body: `{"password": "...", "bank_alias": "..."}`. `password` confirma que es una acción deliberada del dueño de la cuenta (mismo patrón que cambiar email). `bank_alias` es obligatorio si el usuario no tiene uno ya guardado en el perfil; si lo manda, también actualiza el perfil. `400` si falta el alias o tiene formato inválido, `401` si la contraseña no coincide, `409` si ya es entrenador.
- `DELETE /api/v1/users/:id/trainer-role` — desactiva el rol. Self-only. Bloqueado con `409 Conflict` mientras el usuario siga siendo entrenador (`RoleInTeam`) de algún equipo activo — primero hay que transferir o eliminar esos equipos.

## 6. Búsqueda de usuarios (nuevo)

`GET /api/v1/users/search?q=<texto>` — resuelve el gap 1 de `paceron-frontend/docs/BACKEND_API_GAPS.md` (autocompletar al invitar). Cualquier usuario logueado puede usarlo, sin restricción de rol adicional.

- `q` mínimo 3 caracteres (recortando espacios) → `400` si no llega al mínimo.
- Busca coincidencia parcial case-insensitive en nombre, apellido o email, solo entre usuarios con `status = active`.
- Devuelve hasta 5 resultados, cada uno con `user_id`, `name`, `surname`, `email` — deliberadamente acotado (sin DNI, teléfono, dirección ni otros datos sensibles). Si más adelante hace falta un dato extra para el flujo de invitación, se suma al mismo DTO.

## 7. Lookup en lote de usuarios (nuevo)

`GET /api/v1/users?ids=1,2,3` — resuelve el gap 2 de `paceron-frontend/docs/BACKEND_API_GAPS.md` (el roster de equipo/grupo solo trae `user_id`, obligando a un fan-out N+1 contra `GET /auth/user?id=` por cada corredor único). Cualquier usuario logueado puede usarlo, sin restricción de rol adicional.

- `ids` separados por coma, hasta 50 por llamada → `400` si falta el parámetro, si algún id no es numérico, o si se piden más de 50.
- Sin filtro de `status` (a diferencia de `/users/search`): un id ya es un miembro conocido de un equipo/grupo, no un resultado de búsqueda arbitraria — se resuelve igual esté activo, pausado, etc.
- Mismo shape de resultado que `/users/search`: `user_id`, `name`, `surname`, `email` por cada id encontrado. Ids inexistentes simplemente no aparecen en `results` (no es un 404 por id).

## 8. Limitación conocida (no resuelta en esta iniciativa)

Los endpoints de catálogo (`/api/v1/roles`, `/api/v1/tiers`, `/api/v1/permissions`, `/api/v1/auth/permissions`) ahora exigen estar logueado, pero **cualquier usuario autenticado puede gestionarlos** — no hay chequeo de rol especial tipo "admin", porque ese concepto no existe hoy en el dominio (uno está planeado a futuro, fuera del MVP, para moderación tipo baneos/soporte — no reemplaza esto). Documentado como deuda conocida, no como bug.
