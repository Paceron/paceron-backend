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

- **Solo entrenador del equipo**: crear/actualizar/borrar equipo, actualizar dirección, agregar usuarios al equipo, crear/actualizar/borrar grupo, agregar usuarios a un grupo, invitar corredores. Si un corredor (o alguien sin membresía) intenta estas acciones, `403 Forbidden`.
- **Self o entrenador delegado**: sacarse a sí mismo de un equipo/grupo, o que el entrenador saque a otro. Cualquier otro intento (ej. un corredor tratando de sacar a otro corredor) → `403 Forbidden`.
- **Self-only**: actualizar el propio usuario, cambiar el propio estado, cambiar la propia contraseña. Intentar modificar el usuario de otro (aun autenticado) → `403 Forbidden`.

Nuevo `code` en las respuestas de error para estos casos: `"Forbidden"`, `status_code: 403`.

## 5. Limitación conocida (no resuelta en esta iniciativa)

Los endpoints de catálogo (`/api/v1/roles`, `/api/v1/tiers`, `/api/v1/permissions`, asignación de roles a usuarios, `/api/v1/auth/permissions`) ahora exigen estar logueado, pero **cualquier usuario autenticado puede gestionarlos** — no hay chequeo de rol especial tipo "admin", porque ese concepto no existe hoy en el dominio. Documentado como deuda conocida, no como bug.
