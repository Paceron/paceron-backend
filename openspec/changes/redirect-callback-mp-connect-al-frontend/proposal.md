## Why

`GET /api/v1/mercadopago/connect/callback` es la URL que Mercado Pago abre en el **navegador
del entrenador** al terminar la autorización OAuth. Hoy ese endpoint responde
`ctx.JSON(200, CallbackResponse{...})`, así que el usuario queda parado en una pestaña
mostrando `{"success":true,"message":"..."}` — sin forma de volver a la aplicación, y sin
entender si la conexión salió bien.

No es un problema de formato: es que el callback de un flujo OAuth es una **navegación de
usuario**, no una llamada de API. El contrato correcto para una navegación es un redirect al
lugar donde el usuario estaba.

Sin esto, el frontend no puede cerrar el flujo de "conectar Mercado Pago", que es el paso que
bloquea el alta de perfil de entrenador con cobros por split.

## What Changes

- `HandleCallback` deja de responder JSON y termina con un **302** a una URL del frontend,
  con `?status=success` o `?status=error&reason=<slug>`. El canje del `code`, el cifrado y el
  upsert en `seller_connections` **no cambian** — solo cambia cómo termina la request.
- El `state` pasa de `"<userID>-<tsNanos>"` a `"<userID>-<tsNanos>-<target>"`, con
  `target ∈ {web, app}`. Es el único canal que sobrevive el viaje de ida y vuelta, porque el
  `redirect_uri` registrado en Mercado Pago es fijo y no se toca. Los states de 2 segmentos
  siguen siendo válidos (se asume `web`), así que los emitidos antes del deploy no se rompen.
- `GET /api/v1/mercadopago/connect` acepta `?platform=web|app` para elegir ese `target`.
  Cualquier valor distinto de `app` colapsa a `web`.
- El formato del `state` se centraliza en `domains/mpconnect/state.go` (hoy está disperso
  entre un `fmt.Sprintf` en el service y un `fmt.Sscanf` en `validateState`).
- Dos variables de entorno nuevas: `MP_OAUTH_WEB_RETURN_URL` y `MP_OAUTH_APP_RETURN_URL`.
- Se declaran en `render.yaml` las `MP_OAUTH_*` que hoy existen solo en el dashboard de Render.

## Capabilities

### Modified Capabilities
- `mercado-pago-split`: el escenario "Callback de OAuth exitoso" pasa de "devuelve éxito" a
  "redirige al frontend con el resultado". El resto del requisito (persistencia, cifrado, no
  exponer el access_token) no cambia.

## Non-Goals

- **No** se cambia `MP_OAUTH_REDIRECT_URI` ni nada registrado en el panel de Mercado Pago. El
  salto al frontend es un segundo hop interno que MP nunca ve.
- **No** se firma ni se hace single-use el `state` (ver `design.md`, sección Riesgos
  aceptados). Decisión explícita del equipo.
- **No** se toca `POST /api/v1/users/{id}/trainer-role`: la exigencia de tener MP conectado
  para activar el perfil de entrenador se implementa solo en la UI del frontend.
- **No** se declaran en `render.yaml` las otras env vars faltantes (`TOKEN_ENCRYPTION_KEY`,
  `MERCADOPAGO_*`, `SUPABASE_*_S3_*`) — va en `chore/render-yaml-declare-missing-env-vars`,
  por la regla "un cambio = una rama = un tema".

## Impact

- **Nuevo**: `cmd/api/domains/mpconnect/state.go` + `state_test.go`.
- **Modificado**: `controllers/mp_connect_controller.go` (+ test), `services/mp_connect_service.go`
  (+ test), `config/config.go`, `app/app.go`, `.env.example`, `render.yaml`, `CLAUDE.md`,
  `docs/CU/04-alta-autorizacion-onboarding-ejecucion-e2e.md`.
- **Sin cambios**: `app/url_mappings.go` — el callback ya es público (antes del
  `r.Use(AuthMiddleware())`), que es justo lo que necesita: MP redirige el navegador sin
  header `Authorization`.
- **Breaking para consumidores del callback como API**: no hay ninguno. La única llamada real
  la hace el navegador del usuario redirigido por Mercado Pago.
- **Swagger**: `@Success 200` pasa a `@Success 302` + `@Header Location`; regenerar.
- **Acción manual**: cargar `MP_OAUTH_WEB_RETURN_URL` y `MP_OAUTH_APP_RETURN_URL` en Render
  (ambos services) antes de mergear — si no, el default apunta a `localhost:8081`.
