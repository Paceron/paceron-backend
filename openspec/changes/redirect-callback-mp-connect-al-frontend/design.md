# Diseño — Redirect del callback de mp-connect al frontend

## Problema de fondo: el `redirect_uri` es fijo

Mercado Pago exige que el `redirect_uri` enviado en la autorización coincida **exactamente**
con el registrado en el panel de la aplicación (un mismatch es la causa #1 del error
"Aplicación no está lista", ver `docs/CU/04-alta-autorizacion-onboarding-ejecucion-e2e.md`).
O sea: no se puede variar el destino por request.

Pero el destino **sí** tiene que variar: desde la web hay que volver a un origen HTTPS
(`https://paceron-frontend.vercel.app/mp-connect/callback`), y desde la app nativa a un deep
link (`paceron://mp-connect/callback`), porque el navegador que abre la Custom Tab tiene que
poder devolverle el control a la app.

### Decisión: el destino viaja en el `state`

El `state` es el único parámetro que MP nos devuelve tal cual se lo dimos. Se le suma un
tercer segmento:

```
"<userID>-<timestampNanos>-<target>"      target ∈ {web, app}
```

`GET /mercadopago/connect?platform=web|app` elige el target al emitir el state; el callback lo
lee para decidir a cuál de las dos URLs de configuración redirige.

**Retrocompatibilidad:** un state de 2 segmentos se interpreta como `web`. Los states emitidos
antes del deploy siguen funcionando durante su ventana de 10 minutos, así que no hace falta
coordinar el deploy con nada.

### Alternativa descartada: `return_url` libre por query

Recibir la URL de retorno como parámetro de `GET /mercadopago/connect` sería más flexible,
pero convierte un endpoint público en un **open redirect**: cualquiera podría armar un
`auth_url` que termine devolviendo al usuario a un dominio arbitrario, con la credibilidad de
haber pasado por Mercado Pago. Un enum cerrado de dos valores resuelto contra configuración
del servidor no tiene ese problema.

## Dónde va el redirect: controller, no service

`ctx.Redirect` es una decisión de transporte HTTP. El service sigue devolviendo
`*mpconnect.CallbackResponse` y sigue sin conocer ninguna URL de retorno — respeta la regla de
capas de `.agentics/CONVENTIONS.md` (el service no sabe cómo se entrega su resultado) y deja
el service testeable sin `httptest`.

## Los `reason` son slugs, no el texto del error

`mapCallbackReason` traduce el error del service a un slug ASCII estable (`invalid_state`,
`exchange_failed`, …). Dos motivos:

1. **No filtrar internals a la URL.** El texto de un error puede incluir la respuesta cruda de
   MP; esa URL termina en el historial del navegador y en los logs de acceso de Vercel.
2. **Contrato estable para el frontend.** El front mapea slug → mensaje en español. Si mañana
   se reescribe un `fmt.Errorf`, la UI no se rompe.

Se agrega al lado de `mapMPConnectError`, que queda intacta: la siguen usando `GetAuthURL`,
`GetStatus` y `HandleDeauthWebhook`, que sí son API de verdad y responden JSON.

## Guard: sin URLs configuradas, se comporta como hoy

Si `MP_OAUTH_WEB_RETURN_URL` y `MP_OAUTH_APP_RETURN_URL` están vacías, `HandleCallback`
responde el JSON de siempre. Así un entorno mal configurado degrada a lo anterior en vez de
redirigir a `""` y dejar al usuario en una página en blanco.

## Riesgos aceptados

### El `state` es forjable — decidido: no se arregla en este change

El `state` no se persiste, no es single-use y no está firmado; el único control es el expiry
de 10 minutos. Como el callback es público, un atacante puede:

1. Autorizar su **propia** cuenta de Mercado Pago y capturar su `code` de la barra del navegador.
2. Reenviarlo al callback con un state forjado con el `user_id` de otra persona:
   `?code=<code_propio>&state=<victimaID>-<ahora_en_nanos>-web`.
3. El backend valida formato y expiry (ambos los controla el atacante) y hace el upsert en
   `seller_connections` con `user_id = victimaID` y los tokens de la cuenta del atacante.

**Resultado: los cobros del equipo de la víctima caen en la cuenta del atacante.** Los
`user_id` son enteros secuenciales, o sea adivinables.

Este change **no introduce** el problema ni lo empeora — el `target` agregado es un enum
cerrado resuelto contra config del servidor, no un open redirect. Tampoco lo cierra: el equipo
decidió explícitamente dejarlo documentado por ahora.

**El fix, cuando se encare:** firmar el state con HMAC-SHA256 (cierra la forjabilidad, cabe
entero en `domains/mpconnect/state.go`) y, para cerrar también el reuso, una tabla
`oauth_states` con consumo single-use y binding a la sesión que pidió la `auth_url`.
**Revisar antes de cualquier uso con dinero real.**

### El gate de "entrenador necesita MP conectado" es solo de UI

`POST /api/v1/users/{id}/trainer-role` sigue aceptando la activación sin conexión de MP.
Además hay APKs viejos distribuidos sin ese gate. El día que el cobro con split sea
obligatorio, ese endpoint tiene que validar `seller_connections.status = 'authorized'`.

### Cold start de Render dentro del flujo OAuth

Mercado Pago redirige el navegador al callback; si el backend está dormido (plan free,
20-25s), el usuario ve una pestaña cargando sin explicación. No hay mitigación real en plan
free más que despertar el backend con `GET /ping` antes de arrancar el flujo.
