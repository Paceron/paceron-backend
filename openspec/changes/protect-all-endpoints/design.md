## Context

Con la infraestructura de sesión ya mergeada, falta el paso que la hace real: exigirla. El plan original (ver memoria de la conversación / historial) categorizó cada endpoint existente en un patrón de autorización, basado en releer el código de cada service antes de tocarlo — no en asumir cómo "debería" funcionar.

## Goals / Non-Goals

**Goals:**
- Todas las rutas de negocio detrás de `AuthMiddleware()`, salvo las explícitamente públicas.
- Autorización real (no solo autenticación) en las operaciones que hoy no la tienen.
- Mantener el patrón service-layer para autorización (no middleware genérico) — ya evaluado y descartado en la rama anterior por requerir el recurso cargado (ABAC).

**Non-Goals:**
- Inventar un concepto de "admin" — los endpoints de catálogo quedan detrás de login únicamente, limitación documentada.
- Decidir el destino final de las rutas legacy (`/user/:user_id`, `POST /user`) — quedan públicas sin cambios, señaladas como deuda.
- Tocar el frontend — el contrato nuevo se documenta en `docs/AUTH_MIGRATION.md` para que se consuma en otra iniciativa.

## Decisions

### 1. Claves de contexto en `utils`, no en `app`
**Por qué**: `AuthMiddleware()` vive en `app` y setea `c.Set(...)`; los controllers necesitan leer esos valores pero no pueden importar `app` (que a su vez importa `controllers` — ciclo). Se creó `utils.AuthUserIDKey` + `utils.GetAuthUserID(c)` como punto de verdad único, y `app/middleware.go` se actualizó para usar las mismas constantes en vez de duplicarlas.

### 2. `callerID` como parámetro explícito, no leído del contexto dentro del servicio
**Por qué**: los services no reciben `*gin.Context` como "bolsa de todo" a propósito — siguen recibiéndolo solo porque otros métodos ya lo usaban así (logging, cancelación). La identidad autenticada se resuelve en el controller (la única capa que sabe que existe un "request HTTP" con headers) y se pasa explícita a los métodos de servicio que la necesitan para autorizar. Mantiene los services testeables sin tener que simular un `gin.Context` con valores seteados.

### 3. Patrones de autorización van por caso de uso, no por archivo
Confirmado leyendo cada service antes de tocarlo (no asumido):
- **Solo entrenador**: operaciones administrativas del equipo/grupo sin equivalente "self" razonable (nadie se auto-agrega a un equipo sin invitación; nadie se auto-actualiza la dirección del equipo).
- **Self o entrenador delegado**: acciones donde el propio afectado tiene el derecho natural de actuar sobre sí mismo (salir de un equipo/grupo), y el entrenador tiene autoridad delegada sobre otros.
- **Self-only**: datos personales del usuario — nadie más, ni el entrenador, puede modificarlos.

### 4. `team.Create`: `owner_id` sale del body, entra como parámetro de servicio
**Por qué**: si el owner sigue siendo un campo del body, cualquier usuario logueado podría fundar un equipo "a nombre de" otro (con el ID de otro entrenador). El único owner válido es quien está autenticado.

### 5. Accept/Reject de invitaciones sin body
**Por qué**: el único dato que llevaba el body (`user_id`) ahora sale del token. Un endpoint POST sin body es válido en REST cuando la acción no necesita parámetros además de la identidad y el recurso en la URL.

### 6. `GET /api/v1/groups` sin `user_id` query cuando se filtra por `team_id`
**Por qué**: antes el chequeo de membresía usaba un `user_id` de query controlado por el cliente — se podía consultar membresía de cualquier otro usuario. Ahora siempre valida al usuario autenticado.

## Risks / Trade-offs

- **Catálogo sin chequeo de rol**: cualquier usuario logueado puede crear/editar/borrar roles, tiers y permisos globales. Aceptado como limitación conocida — el dominio no tiene hoy un concepto de rol de plataforma separado del rol de negocio (entrenador/corredor), y no corresponde inventarlo en esta iniciativa.
- **Rutas legacy sin proteger**: `/user/:user_id` y `POST /user` quedan públicas. Bajo riesgo — son rutas duplicadas de versiones `/api/v1` ya protegidas, sin indicios de uso real por el frontend actual.
- **Breaking change de contrato**: `LoginResponse` (rama anterior) y ahora la eliminación de `owner_id`/`user_id` de varios endpoints son cambios incompatibles con el frontend actual. Aceptado a propósito — se documenta en `docs/AUTH_MIGRATION.md` para que el frontend se actualice en una iniciativa separada, después de mergear esta rama.
