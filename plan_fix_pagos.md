# Plan de Fix — Pago de Suscripción de Equipo con Split (Mercado Pago)

> Documento de contexto para retomar el trabajo en cualquier momento, aunque caduquen los tokens/códigos de diagnóstico.

- **Última actualización:** 2026-09-06 · branch `develop` (actual: `b60dc28`)
- **Estado:** diagnóstico completo, fix **sin implementar todavía**.
- **Problema a resolver:** el E2E de "pago de participación de equipo con split" falla al aprobar el pago en Mercado Pago con error `400 Invalid users involved (código 2034)`. Y el E2E **sin split** (pago de suscripción individual / cambio de tier) ya funciona y **no debe romperse**.

---

## 1. Qué estamos intentando resolver

El flujo de negocio: un corredor (user 4) paga la cuota de membresía del equipo (team 20, owner = entrenador user 3, fee `1500`). El dinero debe entrar a la cuenta de Mercado Pago del **entrenador** (vendedor) y Paceron (marketplace) cobra su comisión (`marketplace_fee`, hoy 5%). Esto es un **pago con split** (modelo marketplace de MP vía OAuth mp-connect).

E2E objetivo (pasos Bruno, colección `CU pago participacion equipo/`):
1. `1 - Login miembro` → JWT de user 4.
2. `2 - Descubrir mi equipo` / `3 - Leer mi suscripcion de equipo` → team 20, membership_fee 1500.
3. `4 - Crear preferencia` (concept `team_subscription`) → `preference_id`.
4. `5 - Obtener token de tarjeta` → card token (aquí está el bug).
5. `6 - Procesar pago` → **MP debe aprobar** (hoy devuelve 500 genérico escondiendo 2034).
6. Webhook → cuota #5 `paid` → team_user 27 `subscription_status=active`.
7. `8 - Verificar membresia activa`.

### Error exacto observado

- Llamada directa a `POST https://api.mercadopago.com/v1/payments` con el **access token OAuth del vendedor** (test seller bajo app Paceron):
  `HTTP 400 · status: rejected · error: bad_request · status_detail: "Invalid users involved" · code: 2034`.
- Cuando el collector era el **owner/integrador** (`40671376`), el pago **se creaba** pero quedaba `pending_contingency` (nunca aprobaba). → Escenario "vendedor = integrador" (pagar a sí mismo).
- El backend esconde el error real: `ProcessPayment` devuelve error y el controller responde `500 Error al procesar el pago`.

### Causa raíz (confirmada leyendo el código)

**`GenerateCardToken` hardcodea el Public Key del integrador**. En `cmd/api/restclients/mercadopagoclient/client.go:186-238`, el método recibe `accessToken` pero lo **ignora**: usa `appconfig.MyMP.PublicKey` para la URL de `/v1/card_tokens?public_key=...`.

Regla de coherencia de MP (documentada por el equipo/MP):
> En pagos con tarjeta, el **card token** (generado con un Public Key) debe ser coherente con el **Access Token** que se usa para crear el pago.

| Flujo | Collector | Card token | Pago | Resultado |
|---|---|---|---|---|
| **A — sin split** (suscripción individual / cambio de tier) | Integrador | PK integrador | AT integrador (`s.accessToken`) | ✅ funciona |
| **B — con split** (`team_subscription`) | Vendedor | PK integrador (**mal**) | AT vendedor (`resolveTeamSplitConfig`) | ❌ `2034 Invalid users involved` |

Fix: en el **Flujo B** el card token debe generarse con el **Public Key del vendedor**.

---

## 2. Cambios propuestos (fix backend, no toca Flujo A)

1. **Guardar el Public Key del vendedor**:
   - `OAuthTokenResponse` (`client.go:61-68`) no captura `public_key`, pero MP lo devuelve en la respuesta de `POST /oauth/token`. Agregar campo `PublicKey string \`json:"public_key"\`` y propagarlo desde `ExchangeCodeForToken`.
   - Agregar columna `public_key` en la tabla `seller_connections` (migración).
   - En `HandleCallback` (`services/mp_connect_service.go:78`) guardar `tokenResp.PublicKey` (o el del refresh) en `dbs.SellerConnection.PublicKey`.

2. **`GenerateCardToken`** (`client.go:186`): dejar de hardcodear `appconfig.MyMP.PublicKey`. Usar el `publicKey`/token que recibe (misma firma, misma petición — el AT del vendedor ya se recibe como primer parámetro; resolver el PK a partir de su cuenta o pasar un `publicKey` por parámetro).

3. **`resolveTeamSplitConfig`** (`services/payment_service.go:659`): además del access token, devolver el `public_key` del vendedor al procesar pagos `team_subscription`.

4. **`GenerateTestCardToken` / llamada a card token** (`payment_service.go:731`): si el pago es `team_subscription`, generar el card token con el PK del vendedor; si no, seguir con el PK del integrador (como hoy).

5. **`CreatePreference`** (`payment_service.go:77`): verificar si el `PublicKey` que devuelve al frontend en `CreatePreferenceResponse` debe ser el del vendedor en flujo B (para que el brick elija bien). Hoy devuelve `s.publicKey` (integrador) siempre.

6. **Tests y migración**:
   - Migración SQL para `seller_connections.public_key` (nullable).
   - Ajustar mocks de tests (`payment_service_test.go`): `GenerateCardToken`/`resolveTeamSplitConfig`.
   - `go test ./...` verde y no romper el Flujo A (pago individual/tier).

### Regla de resolución de credenciales (a implementar en servicio)

```
Si concept == "team_subscription" (split):
    tokenizar con PK del vendedor  →  pagar con AT del vendedor
Sino (individual / tier / order):
    tokenizar con PK del integrador →  pagar con AT del integrador   (Hoy: s.accessToken / s.publicKey)
```

---

## 3. Probes de diagnóstico (temporales, fuera del repo)

Viven en el temp dir de opencode, **no** son parte del repo:

- `/var/folders/f7/pnkdn7hd42l0jf3p8tr5cry00000gp/T/opencode/pgprobe/` — probe SQL a la DB de testing (`SUPABASE_TESTING_DATABASE_URL`). Consultas útiles: `seller_connections`, `installments`, `team_users`, `team_users.subscription_status`.
- `/var/folders/f7/pnkdn7hd42l0jf3p8tr5cry00000gp/T/opencode/mpdiag/main.go` — probe de MP:
  - Lee `seller_connections` de user 3, descifra el access token OAuth con la clave de Render (`RENDER_TOKEN_ENCRYPTION_KEY`).
  - `DUMP_TOKEN=1` → imprime el token descifrado (para tokenizar a mano).
  - Sin flag: hace `POST /v1/payments` directo con el token del vendedor y muestra el error real de MP (evita el 500 genérico del backend).
  - Envs: `REPO_DIR=<ruta repo>` (para `.env`), `RENDER_TOKEN_ENCRYPTION_KEY`, `PAYER_EMAIL`, `CARD_TOKEN`, `WEBHOOK_URL`.
  - Run: `cd <REPO_DIR> && REPO_DIR=<REPO_DIR> RENDER_TOKEN_ENCRYPTION_KEY=<key> go run /var/folders/.../mpdiag/main.go`.

> ⚠️ Próxima vez, recrear el probe si el temp dir se limpió: leer `main.go` actual es la referencia. La clave de cifrado de tokens está solo en Render (env var `TOKEN_ENCRYPTION_KEY`); el probe usa `RENDER_TOKEN_ENCRYPTION_KEY` con el valor que ya se usó en la sesión anterior.

---

## 4. Contexto operativo para retomar

### Instancias
- **Render (backend testing):** `https://paceron-backend-as9c.onrender.com` (cold start ~20-40s, no es error).
- **Supabase testing** y **Supabase production** — ver `docs/ENVIRONMENTS.md`. `develop`/local apuntan a **testing** por default; producción exige `--stage=production`.
- Repo frontend es **otro** (Expo/React Native, otro repo, lo mantiene otro miembro).

### App de Mercado Pago (paceron)
- client_id: `2636114621042686`, owner/account real: `40671376`, site MLA.
- Credenciales (APP_USR y TEST) y **Public Key** del integrador: en `.env` / Render. El PK del integrador es `TEST-9a1e5d38-929e-45f8-ad90-01ca7887fe82`.
- El frontend inicializa el brick con `marketplace: true` y `public_key` del integrador (ver `.agentics/payments-integration.md`).

### Cuentas de usuario de la app (credenciales locales de testing)
- user 3 = entrenador+corredor (tier 3 y 1) → `pepa@lota.com` / `Abcd-12345`.
- user 4 = corredor → `pepae@lote.com` / `Abcd-12345`.

### Test users de Mercado Pago (creados vía API de test users)
| Rol | MP user id | email | pass | App |
|---|---|---|---|---|
| **Seller ACTUAL** (conectado vía OAuth a user 3) | `3665501084` | `test_user_2619844397058197960@testuser.com` | `MPtAELenkb` | paceron (creado con APP_USR de paceron) |
| Seller descartado | `3666809602` | `TESTUSER6788565356996580124@testuser.com` | `H1jUzyBDwK` | app del MCP (no paceron) |
| Buyer | `3665501076` | `test_user_3762299195684310626@testuser.com` | `lMMopL25SM` | paceron |
| Buyer (MCP) | `3665799579` | `TESTUSER6115052701798260193` | — | app del MCP |

> Conectar user 3 al seller paceron (`3665501084`) requiere **reconfirmar el OAuth en browser**: `GET /api/v1/mercadopago/connect?user_id=3`, autorizar, y el callback guarda `seller_connections` (user_id 3 → mp_user_id, status `authorized`).

### Datos de prueba (pagos de ejemplo — quedan en `pending_contingency`, no cuentan)
- Pago MP `1351499917` (local id 67, collector `40671376`, payer `pepae@lote.com`).
- Pago MP `1328083040` (local id 71, collector `40671376`, payer `comprador.e2e.2026@example.com`) — **este formato de email arbitario sirve** para simular el payer en sandbox.
- Cuota activa objetivo: `installments` id 5 (cuota #1, amount 1500, `pending`), team_user id 27 (user 4 → team 20, `first_payment_pending`).
- Errores ya vistos: `403 Payer email forbidden (4390)` con emails `@testuser.com` como payer con AT del integrador (usar email arbitrario, NO `@testuser.com`); `400 Invalid users involved (2034)` por mezclar PK integrador + AT vendedor.
- Tarjetas de test: **visa `4509 9535 6623 3704` / master `5031 7557 3453 0604`**, exp `11/2030`, CVV `123`, titular `APRO Test User`, DNI `12345678`.

---

## 5. Referencias útiles del repo

### Documentación
- `.agentics/payments-integration.md` — **la guía de integración completa (español/inglés)**. Sección "Split payments (marketplace)" documenta OAuth, marketplace_fee, y el flujo Bricks con split. Leer antes de implementar.
- `.agentics/CONVENTIONS.md` / `.agentics/STRUCTURE.md` / `.agentics/WORKFLOW.md` — convenciones de capas y cómo agregar features.
- `docs/CU/02-pago-participacion-equipo.md` — CU del pago de participación de equipo (spec).
- `docs/CU/03-cambio-de-tier-ejecucion-e2e.md` — E2E del pago individual (Flujo A, **la referencia que NO hay que romper**).
- `docs/CU/04-alta-autorizacion-onboarding-ejecucion-e2e.md` — E2E de conexión OAuth (reconectar seller).
- `docs/ENVIRONMENTS.md` — stages Supabase testing/production, env vars, checklist Render.
- `docs/TESTING.md` — cómo correr tests (`go test ./...`), DB real, coverage.
- `docs/PAYMENT_TESTING.md` / `docs/CAMBIO_SUSCRIPCION_TEAMS_SPLIT_TESTING.md` / `docs/CAMBIO_TIER_SUBSCRIPTION_TESTING.md` — guías de testing de pagos/suscripciones.

### Endpoints / colecciones Bruno
- `endpoint-collections/CU pago participacion equipo/` — **colección del flujo con split** (pasos 0-8, incluye `[Opcional] Simular webhook MP`).
- `endpoint-collections/CU cambio de tier/` — flujo **sin split** (referencia que funciona).
- `endpoint-collections/CU alta autorizacion campana/` — flujo OAuth (conectar entrenador).
- `endpoint-collections/payments bruno collections/` — colección base de payments (login, preferencia, procesar pago, webhook).

### Código afectado
- `cmd/api/restclients/mercadopagoclient/client.go` — `OAuthTokenResponse` (61), `ExchangeCodeForToken`, `CreatePreference` (90), `CreatePayment` (123), `GenerateCardToken` (186).
- `cmd/api/services/mp_connect_service.go` — `HandleCallback` (78), guarda `seller_connections`.
- `cmd/api/services/payment_service.go` — `CreatePreference` (77), `ProcessPayment` (160), `resolveTeamSplitConfig` (659), `GenerateTestCardToken` (731).
- `cmd/api/services/payment_service_test.go` — mocks de `GenerateCardToken`, `CreatePreference`, etc.
- `cmd/api/daos/` (seller_connection DAO + migraciones de schema).

---

## 6. Cómo validar el fix (checklist final)

1. Código en rama dedicada (`fix/<kebab-case>` desde `develop`), migración aplicada en testing.
2. Reconectar (si hace falta) el OAuth de user 3 al **test seller paceron `3665501084`** → `seller_connections` con `public_key` guardado.
3. Correr la colección `CU pago participacion equipo/` (pasos 1→8):
   - `5 - Obtener token de tarjeta` debe tokenizar con el **PK del vendedor**.
   - `6 - Procesar pago` con `payer` email **arbitrario** (ej. `comprador.e2e.2026@example.com`) → MP debe aprobar (`approved`).
4. Webhook (o `7 - [Opcional] Simular webhook MP`) → cuota id 5 `paid`, team_user 27 `subscription_status=active`.
5. `go test ./...` verde (incluye Flujo A: cambio de tier no afectado).
6. Opcional: verificar split — `marketplace_fee` en DB (`payments.marketplace_fee`) e `fee_details` en la respuesta de MP.

---

## 7. Pendientes / decisiones abiertas

- **Frontend:** el brick hoy usa el `public_key` del integrador con `marketplace: true` (ver `.agentics/payments-integration.md`). Si MP exige el PK del vendedor también en Bricks para split, hay que coordinar un cambio en el repo frontend. Para el E2E server-to-server de Bruno **no hace falta** tocar frontend.
- **¿El PK que devuelve `CreatePreferenceResponse` debe ser el del vendedor en flujo B?** Decidir en la implementación según lo que pida el brick (ver paso 5 de la sección de cambios).
- **Manejo de errores:** considerar devolver/loguear el error real de MP (status_detail/code) en vez de `500 Error al procesar el pago` para facilitar diagnósticos futuros.