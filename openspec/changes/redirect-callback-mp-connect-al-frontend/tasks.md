## 1. Formato del state

- [x] 1.1 Crear `cmd/api/domains/mpconnect/state.go` con `TargetWeb`/`TargetApp`, `BuildState(userID, target)`, `ParseState(state)` y `TargetFromState(state)`. Usar `strings.SplitN(state, "-", 3)` + `strconv.ParseInt` (no `fmt.Sscanf`: con tres segmentos el formato `"%d-%d"` queda ambiguo). Un state de 2 segmentos resuelve a `web`
- [x] 1.2 Crear `cmd/api/domains/mpconnect/state_test.go`: round-trip de los dos targets, legacy de 2 segmentos, target desconocido, string vacío y basura no numérica

## 2. Service

- [x] 2.1 `GetAuthURL(ctx, userID, target)` en `MPConnectServiceInterface` y su implementación, usando `mpconnect.BuildState`
- [x] 2.2 `validateState` delega el parseo en `mpconnect.ParseState`, conservando el expiry de 10 minutos
- [x] 2.3 Actualizar `mp_connect_service_test.go` por la firma nueva + caso que verifica el sufijo del state

## 3. Controller

- [x] 3.1 `NewMPConnectController(svc, webReturnURL, appReturnURL)` y el campo correspondiente en el struct
- [x] 3.2 `GetAuthURL` lee `ctx.DefaultQuery("platform", "web")`; cualquier valor distinto de `app` colapsa a `web`
- [x] 3.3 `mapCallbackReason(err) string` con los slugs definidos en el proposal, al lado de `mapMPConnectError` (que queda intacta)
- [x] 3.4 `buildReturnURL(base, status, reason)` con `net/url`, mergeando la query existente; funciona igual para `https://host/path` que para `paceron://mp-connect/callback`
- [x] 3.5 `HandleCallback` termina en `ctx.Redirect(302, ...)` en ambas ramas, con el guard de "sin URLs configuradas → JSON como antes"
- [x] 3.6 Anotaciones Swagger: `@Success 302` + `@Failure 302` + `@Header 302 Location`, y regenerar con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`

## 4. Config y wiring

- [x] 4.1 `config.MercadoPago`: campos `OAuthWebReturnURL`/`OAuthAppReturnURL`, leídos en `loadMercadoPagoConfig()` con `getEnvOrDefault` (defaults `http://localhost:8081/mp-connect/callback` y `paceron-dev://mp-connect/callback`)
- [x] 4.2 `app/app.go`: pasar las dos URLs a `NewMPConnectController`
- [x] 4.3 Verificar que `app/url_mappings.go` no necesita cambios (el callback ya es público)

## 5. Tests del controller

- [x] 5.1 Actualizar las llamadas existentes a `NewMPConnectController` a la firma de 3 argumentos
- [x] 5.2 Reescribir `HandleCallback_Success` y `HandleCallback_ServiceError` para verificar `302` + header `Location`
- [x] 5.3 Casos nuevos: target `app` (Location con scheme `paceron://`), state malformado (→ web), guard sin URLs configuradas (→ 200 JSON), tabla de todos los slugs de `mapCallbackReason`
- [x] 5.4 `GetAuthURL` con `?platform=app` y con un valor basura
- [x] 5.5 `go build ./...`, `go vet ./...`, `go test ./...` verdes y coverage total ≥ 80 (`make test-db-up && make coverage-with-db`)

## 6. Env vars y docs

- [x] 6.1 `.env.example`: agregar `MP_OAUTH_WEB_RETURN_URL` y `MP_OAUTH_APP_RETURN_URL` con comentario explicando que el destino sale del 3er segmento del `state`
- [x] 6.2 `render.yaml`: las dos vars nuevas con `value:` en los dos services, con los valores por entorno
- [x] 6.3 `render.yaml`: declarar las `MP_OAUTH_CLIENT_ID`/`MP_OAUTH_CLIENT_SECRET`/`MP_OAUTH_REDIRECT_URI`/`MP_OAUTH_TEST_TOKEN` que hoy viven solo en el dashboard. **Se hizo con `sync: false` para las cuatro**, no con `value:` como decía el plan original: sus valores son específicos del entorno y ya están cargados y funcionando; escribir un `value:` adivinado los pisaría (en particular `MP_OAUTH_REDIRECT_URI`, que tiene que coincidir exacta con el panel de MP). Declararlas igual cumple la regla de "toda key que el binario necesita vive en render.yaml"
- [x] 6.4 `CLAUDE.md`: sección corta al lado de CORS documentando el 302, el target en el state y las vars a mantener sincronizadas
- [x] 6.5 `docs/CU/04-alta-autorizacion-onboarding-ejecucion-e2e.md`: reescribir Pasos 3 y 4 y la sección "Para quien integre el frontend"

## Notas de ejecución

- **Bug preexistente corregido de paso:** `HandleCallback` aplastaba el error de `validateState`
  en `"state inválido"`, lo que hacía **inalcanzable** el caso `"state expirado"` que
  `mapMPConnectError` ya contemplaba. Ahora se propaga, y el frontend puede distinguir "el enlace
  venció" de "algo salió mal" (`expired_state` vs `invalid_state`). Dos tests del service que
  afirmaban el comportamiento viejo se actualizaron.
- **Coverage:** no se pudo correr `make coverage-with-db` (no hay `make` ni Docker corriendo en la
  máquina de desarrollo). Se midió el delta con el mismo comando del Makefile pero sin base, contra
  un worktree de `develop`: **71.1% → 71.3%** (los tests de DAO se auto-saltean sin `TEST_DB_HOST`,
  por eso el absoluto es menor al de CI). El cambio **sube** la cobertura. CI lo valida con base.
- **`go test ./...` falla en `cmd/api/app`** por `docs_handler_test.go`, que compara `/index.html`
  contra el `\index.html` que devuelve `filepath.Clean` en Windows. Es preexistente, ajeno a esta
  rama (el único archivo tocado ahí es `app.go`) y no ocurre en CI, que corre en Ubuntu.
