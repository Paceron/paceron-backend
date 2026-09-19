# CU · Alta de autorización para cobrar (onboarding del entrenador) — E2E 100% backend + Bruno

> Paso a paso para probar **todo por backend vía Bruno** (sin frontend), y para dejarle a quien
> integre el frontend las instrucciones de cómo conectar el alta de autorización MP de un entrenador.

La base URL usada en los ejemplos es el deploy de **testing en Render**:

```
https://paceron-backend-as9c.onrender.com
```

Si querés correr local con ngrok, reemplazala por tu URL de ngrok en los requests de Bruno.
**En Bruno: los requests apuntan a `{{BASE_URL}}`** → definí la variable `BASE_URL` en la colección.

---

## Objetivo

Un **entrenador** autoriza a la plataforma a cobrar en **su** nombre (su cuenta de Mercado Pago queda
como vendedor en los pagos de participación de equipo, con comisión de Paceron). Es la precondición
**P2** del CU de pago de participación: sin conexión del entrenador, el split no puede crear preferencia.

Esto es OAuth 2.0 con **Mercado Pago** (`auth.mercadopago.com/authorization` + `POST /oauth/token`).
El entrenador **nunca** nos da su password ni su access_token: solo autoriza a la app, y el backend
guarda los tokens de su conexión **cifrados** en `seller_connections`.

---

## Pre-requisitos (una sola vez)

| Qué | Dónde | Notas |
|---|---|---|
| App de Mercado Pago con OAuth habilitado | Mercado Pago Developers → tu app | Anotar **Client ID** y **Client Secret** |
| URL de redirección registrada | App MP → Redirigir URLs | Debe ser **exacta**: `https://paceron-backend-as9c.onrender.com/api/v1/mercadopago/connect/callback` |
| Env vars en Render (`paceron-backend-as9c`) | Dashboard Render | Ver tabla de abajo |
| **Test seller** (el "entrenador") | Panel MP → Cuentas de prueba | Su email/password se usan para autorizar en el navegador |
| **Test payer** (el "miembro"), con email | Panel MP → Cuentas de prueba | Es el `payer_email` del pago |
| Tarjeta de prueba del payer | Tarjetas de prueba de MP | Visa `4509 9535 6623 3704` o Mastercard `5031 7557 3453 0604` |

### Env vars en Render

| Variable | Valor |
|---|---|
| `MP_OAUTH_CLIENT_ID` | Client ID de la app MP |
| `MP_OAUTH_CLIENT_SECRET` | Client Secret de la app MP |
| `MP_OAUTH_REDIRECT_URI` | `https://paceron-backend-as9c.onrender.com/api/v1/mercadopago/connect/callback` |
| `TOKEN_ENCRYPTION_KEY` | `openssl rand -base64 32` |
| `MP_OAUTH_TEST_TOKEN` | `true` (default; access_token de prueba aunque las credenciales sean de producción) |
| `MERCADOPAGO_WEBHOOK_URL` | `https://paceron-backend-as9c.onrender.com/api/v1/payments/webhook` |

> ⚠️ La `redirect_uri` debe ser **idéntica** en la app MP y en `MP_OAUTH_REDIRECT_URI`. Un mismatch
> (barra final, `http` vs `https`, hash distinto) es la causa #1 del error **"Aplicación no está lista"**.

---

## Colección Bruno

Todo el flujo del caso de uso está en:

```
endpoint-collections/CU alta autorizacion campana/
```

- `1 - Login coach (entrenador).yml`
- `2 - Generar URL de autorizacion OAuth.yml`
- `3 - [Opcional] Reprocesar callback con code capturado.yml`
- `4 - Verificar estado de conexion del entrenador.yml`

> Los requests usan `http://localhost:8080` como base (igual que la colección existente del repo).
> Para apuntar al deploy de testing de Render, definir la variable de colección
> `BASE_URL=https://paceron-backend-as9c.onrender.com` y cambiar las URLs de los requests a
> `{{BASE_URL}}/...`.

`endpoint-collections/CU pago participacion equipo/` tiene el flujo de pago posterior (0 a 8).

---

## Paso a paso (100% backend)

### Paso 1 — Login del entrenador (coach)

**Request Bruno:** `1 - Login coach (entrenador)`

```
POST {{BASE_URL}}/api/v1/auth/login
Body (json):
{ "email": "entrenador@example.com", "password": "Abcd-12345" }
```

Guarda `coach_token` y `coach_user_id` en las variables de colección (ya lo hace el script).

**Chequear:** `200`, `access_token` presente.

> ℹ️ El entrenador del E2E necesita una **password conocida**. Hoy los coaches de testing no la tienen
> estándar; configurarla es parte del setup previo (UPDATE en DB de testing).

### Paso 2 — Generar la URL de autorización OAuth

**Request Bruno:** `2 - Generar URL de autorizacion OAuth`

```
GET {{BASE_URL}}/api/v1/mercadopago/connect
Header: Authorization: Bearer {{coach_token}}
```

**Respuesta:**
```json
{
  "auth_url": "https://auth.mercadopago.com/authorization?client_id=...&response_type=code&redirect_uri=...&state=<coach_user_id>-<timestamp>-<target>",
  "state": "<coach_user_id>-<timestamp>-<target>"
}
```

**Chequear:**
- `200`, `auth_url` y `state` no vacíos.
- El `redirect_uri` dentro de `auth_url` **coincide** con `MP_OAUTH_REDIRECT_URI` y con el de tu app MP.
- El `state` empieza con el `coach_user_id` y termina con el target (`web` o `app`).

El target sale de `?platform=web|app` (default `web`) y define a dónde redirige el backend
después de procesar el callback: al origen web (`MP_OAUTH_WEB_RETURN_URL`) o al deep link de
la app (`MP_OAUTH_APP_RETURN_URL`). Cualquier valor desconocido colapsa a `web`.

**Si da error "configuración de Mercado Pago incompleta":** faltan `MP_OAUTH_CLIENT_ID` o
`MP_OAUTH_REDIRECT_URI` en el backend (Render).

### Paso 3 — Autorizar en el navegador con el Test Seller

Esto NO es por Bruno ni por API (lo hace MP en el navegador). Abrí la `auth_url` en un navegador:

1. Logueate con el **test seller** (cuenta de prueba del "entrenador").
2. Completá la pantalla de autorización (Es Infinity / aprobar).
3. MP redirige el navegador al callback del backend:

```
{{BASE_URL}}/api/v1/mercadopago/connect/callback?code=DGTZ-XXXXXXXX&state=<coach_user_id>-<timestamp>-web
```

4. El backend lo procesa y hace un **segundo salto (302)** hacia el frontend. El navegador
   termina en la URL de retorno, no en el callback:

```
<MP_OAUTH_WEB_RETURN_URL>?status=success
<MP_OAUTH_WEB_RETURN_URL>?status=error&reason=exchange_failed   # si algo falló
```

> Si MP muestra **"Aplicación no está lista"**, es mismatch de `redirect_uri` (ver Pre-requisitos).
> **El `code` ya no queda visible al final del flujo** — el redirect del backend no lo propaga, a
> propósito. Si lo necesitás para dispararlo a mano (Paso 4), copialo de la URL intermedia antes
> de que el navegador siga, o sacalo de los logs. Es de un solo uso y vence rápido.

### Paso 4 — El backend procesa el callback (Exchange)

**No hace falta ejecutar nada manual** si MP redirige al servicio de Render; el callback de Render lo
procesa solo. El request Bruno `3` es opcional y sirve para verificar/pegar el `code` si querés
dispararlo a mano apuntando a un redirect local con ngrok:

```
GET {{BASE_URL}}/api/v1/mercadopago/connect/callback?code=DGTZ-XXXXXXXX&state=<coach_user_id>-<timestamp>
```

Internamente el backend, en `HandleCallback`:
1. Valida el `state` (CSRF: pertenece al coach, expira a los 10 min).
2. Si MP manda `error/error_description` → rechaza.
3. Intercambia `code` por tokens: `POST /oauth/token` con `grant_type=authorization_code` +
   `test_token=true`.
4. **Refresca el token de inmediato** (`grant_type=refresh_token`) para maximizar vigencia.
5. Valida la identidad del vendedor: `GET /users/me` → obtiene su `mp_user_id`.
6. Cifra access/refresh con AES-GCM (`TOKEN_ENCRYPTION_KEY`).
7. `Upsert` en `seller_connections` (`status=authorized`, `token_expires_at`).
8. Redirige (`302`) al frontend con `?status=success`, o `?status=error&reason=<slug>` si algo
   falló. El destino sale del target del `state`.

**Chequear:** la respuesta sea `302` con header `Location` apuntando a la URL de retorno con
`status=success` (con `curl -i` se ve sin seguir el redirect), y que el `code` **no** aparezca en
ese `Location`. En los **logs** del backend de Render,
ver los `[DEBUG]` del flujo (con secretos ofuscados): `Token exchange OK`, `Token refresh OK`,
`User info OK`, `Seller connection guardada`.

> Si el backend responde `200` con JSON en vez de `302`, es que `MP_OAUTH_WEB_RETURN_URL` /
> `MP_OAUTH_APP_RETURN_URL` no están configuradas: el callback degrada al comportamiento anterior
> a propósito, para no redirigir a una URL vacía.

### Paso 5 — Verificar estado de conexión

**Request Bruno:** `4 - Verificar estado de conexion del entrenador`

```
GET {{BASE_URL}}/api/v1/mercadopago/connect/status
Header: Authorization: Bearer {{coach_token}}
```

**Esperado:**
```json
{ "connected": true, "account_status": "authorized" }
```

**En DB (opcional, con el probe SQL):**
```sql
SELECT user_id, mp_user_id, status, token_expires_at
FROM seller_connections
WHERE user_id = <coach_user_id>;
-- status = 'authorized'
```

---

## Qué verificar en Mercado Pago (panel devs / test)

- El vendor del pago queda como **test seller** (no el dev de Paceron).
- La notificación de pago llega al webhook de Render (por eso `MERCADOPAGO_WEBHOOK_URL`).
- Las tarjetas de prueba del payer decodifican como **APRO** (pago aprobado).

---

## Qué verificamos acá (backend) y qué se puede ver de la cuenta MP

| Paso | Chequear | Comando/dónde |
|---|---|---|
| Login coach | `200`, token | Bruno `1` |
| auth_url | `redirect_uri` correcto, `state` = coach | Bruno `2` |
| Autorización | el `code` aparece en el callback | navegador + panel MP |
| Exchange/refresh | `[DEBUG]` en logs, sin secretos en claro | logs de Render |
| Conexión guardada | `status=authorized` | DB `seller_connections` / Bruno `4` |
| Split online | `connected: true` | Bruno `4` |

---

## Tan pronto como quede conectado → seguí con el pago

El flujo de pago de participación está en `endpoint-collections/CU pago participacion equipo/`
(0 a 8): login miembro → descubrir equipo → leer suscripción → crear preferencia → token tarjeta →
procesar pago → webhook → verificar membresía.

La preferencia de `team_subscription` usa el token del entrenador (`resolveTeamSplitConfig`) y aplica
el `marketplace_fee` (default 5%) → el pago queda a nombre del **test seller** y la membresía del
miembro pasa a `active`.

---

## Para quien integre el frontend (alta de autorización)

El backend expone el flujo completo; el frontend solo necesita:

1. **Pedir la URL** con el entrenador logueado, declarando desde dónde abre el flujo:
   `GET /api/v1/mercadopago/connect?platform=web|app` → `{ auth_url, state }`.
   El `platform` define a dónde vuelve el navegador al final; default `web`.
2. **Abrir `auth_url`**. En web, una ventana emergente (`window.open`) conserva la pantalla que el
   usuario tenía abierta. En nativo, **Chrome Custom Tabs / Safari View Controller** — Mercado Pago
   [deprecó el login dentro de WebView embebida](https://www.mercadopago.com.ar/developers/es/news/2023/11/30/WebView-integrations-have-been-deprecated)
   (discontinuado el 10/12/2024), así que un `<WebView>` propio **no sirve** para este paso.
   El `state` viaja en la URL; no hace falta guardarlo en el front, lo valida el backend.
3. **MP redirige al backend** (`MP_OAUTH_REDIRECT_URI`) con `code`+`state`. El backend completa el
   alta (exchange → refresh → cifrar → `seller_connections`) y **redirige (302) de vuelta al
   frontend**, a `MP_OAUTH_WEB_RETURN_URL` o `MP_OAUTH_APP_RETURN_URL` según el target del `state`:

   ```
   <URL de retorno>?status=success
   <URL de retorno>?status=error&reason=<slug>
   ```

   Los `reason` posibles: `bad_request`, `missing_params`, `invalid_state`, `expired_state`,
   `authorization_denied`, `config_error`, `exchange_failed`, `save_failed`, `unknown_error`.
   Son un contrato estable — mapealos a un mensaje en español en el front, no muestres el slug.
4. **Confirmar contra el backend**, siempre: `GET /api/v1/mercadopago/connect/status` →
   `connected` / `account_status`. El `status` de la query sirve para el mensaje inmediato, pero la
   fuente de verdad es este endpoint — así el flujo no se rompe si el canal de retorno falla
   (ventana emergente bloqueada, deep link que no resuelve, usuario que cierra a mitad).

Reglas clave:

- **El `code` del OAuth nunca pasa por el frontend.** El callback va directo al backend, y el
  redirect de vuelta no lo propaga.
- **Las URLs de retorno no se registran en Mercado Pago.** MP solo conoce `MP_OAUTH_REDIRECT_URI`
  (el backend); el salto al frontend es interno.
- Si el entrenador cancela o cierra la ventana, no llega ningún redirect: consultá `status` igual.
