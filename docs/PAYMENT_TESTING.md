# Pruebas de Pagos — Guía paso a paso

> Cómo probar la integración de pagos de punta a punta, desde la configuración inicial hasta verificar que el webhook actualiza la DB.

---

## Cómo funciona el flow

```
 ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
 │  Login   │────▶│ Preferen-│────▶│  Procesar │────▶│ Webhook  │────▶│ Verificar│
 │  (token) │     │ cia (MP) │     │  Pago     │     │  (MP)    │     │  DB      │
 └──────────┘     └──────────┘     └──────────┘     └──────────┘     └──────────┘
```

1. **Login** — obtenés el token de autenticación.
2. **Crear preferencia** — el backend le dice a Mercado Pago "voy a recibir un pago de X por Y".
3. **Procesar pago** — el backend envía los datos de la tarjeta (token generado por el Brick en frontend) y MP procesa el cobro.
4. **Webhook** — MP notifica al backend cuando el pago cambia de estado (aprobado, rechazado, pendiente).
5. **Verificar** — revisás que la tabla `payments` tenga el estado correcto.

En producción, el paso 2 lo hace el frontend (Payment Brick). Para testing sin frontend, lo hacemos directamente con Bruno.

---

## Pre-requisitos

| Qué | Por qué | Dónde |
|---|---|---|
| Cuenta de Mercado Pago | Para obtener credenciales de sandbox | https://www.mercadopago.com.ar/developers |
| App `paceron` configurada | Es la app que recibe los pagos | Panel de developers → tu app |
| ngrok (opcional) | Para que MP pueda enviarte webhooks en local | https://ngrok.com |
| Bruno (opcional) | Para correr los requests de prueba | https://www.usebruno.com |

---

## Paso 1: Credenciales

Necesitás el **Access Token** y la **Public Key** de homologación (sandbox). Estos son los valores que le permiten al backend hablar con la API de Mercado Pago.

**Dónde encontrarlos:** Panel de tu app → pestaña **Credenciales** o **Homologación**. El Access Token empieza con `TEST-` (sandbox) o `APP_USR-` (producción). Para testing, usá siempre el de sandbox.

**Cómo setearlos:** Agregá estas líneas a tu `.env`:

```bash
MERCADOPAGO_ACCESS_TOKEN=TEST-tu-token-aqui
MERCADOPAGO_PUBLIC_KEY=TEST-tu-public-key-aqui
MERCADOPAGO_WEBHOOK_SECRET=tu-secret-aqui
MERCADOPAGO_WEBHOOK_URL=https://tu-backend.ngrok.io/api/v1/payments/webhook
MERCADOPAGO_CURRENCY_ID=ARS
```

> **¿Por qué `MERCADOPAGO_WEBHOOK_URL`?** Mercado Pago necesita una URL pública donde enviar las notificaciones de pago. En local, usás ngrok para exponer tu backend. En producción, es la URL de Render.

---

## Paso 2: Test users

Para probar pagos sin mover dinero real, Mercado Pago te permite crear **usuarios de prueba** que tienen saldo ficticio.

**Dónde crearlos:** Panel de tu app → pestaña **Test users** → **Crear test user**.

**Qué crear:**
- **Un comprador** (payer) — tiene saldo de prueba y puede pagar.
- **Un vendedor** — solo si vas a probar split payments (iteración 2).

**Por qué los necesitás:** Cuando procesás un pago en sandbox, el `payer_email` tiene que ser el email de un test user válido. Si usás un email inventado, MP lo rechaza.

---

## Paso 3: Webhook

El webhook es cómo Mercado Pago le avisa a tu backend que un pago cambió de estado. Sin él, tu sistema nunca se entera si un pago fue aprobado o rechazado.

**Qué registrar:** En el panel de tu app → pestaña **Webhooks** → **Crear webhook**.

- **URL:** tu `MERCADOPAGO_WEBHOOK_URL`
- **Events:** seleccioná `payment`

**El secret:** MP te genera un secreto cuando creás el webhook. Copialo a `MERCADOPAGO_WEBHOOK_SECRET`. Este secreto se usa para validar que las notificaciones realmente vienen de MP y no de un atacante.

> **¿Por qué importa?** Si un atacante te envía un webhook falso diciendo "pago aprobado", tu sistema podría entregar un servicio sin haber cobrado. La validación de firma previene eso.

---

## Paso 4: Probar

Abrí la colección de Bruno en `endpoint-collections/payments bruno collections/` y corridalos en orden:

### 4.0 — Login

Corré `00_login.yml`. Copiá el `access_token` de la respuesta y pegalo en la variable `token` de Bruno (collection level → Variables → token).

> **¿Por qué?** Todos los endpoints de pagos requieren autenticación. El token vence en 15 minutos.

### 4.1 — Crear preferencia

Corré `01_crear_preferencia.yml`. La respuesta te devuelve un `preference_id` y un `public_key`.

**Qué verificar:** que la respuesta tenga status 201 y los campos `preference_id` y `public_key` no estén vacíos.

> **¿Qué hace?** Le dice a Mercado Pago "voy a recibir un pago de 1000 ARS por un item llamado Test Item". MP reserva ese importe y te da un ID para rastrearlo.

### 4.2 — Procesar pago

Pegá el `preference_id` del paso anterior en el body de `02_procesar_pago.yml` y corrilo.

**Qué verificar:** que la respuesta tenga `status: "approved"` o `status: "pending"` (pending es normal si el pago requiere autenticación adicional).

> **¿Por qué el `token` de tarjeta?** En producción, el Payment Brick genera este token automáticamente (nunca ves los datos de la tarjeta). Para testing sin frontend, necesitás generar el token con el SDK de MP o usar un endpoint de testing.

### 4.3 — Webhook

Abrí el panel de MP → tu app → Webhooks → tu webhook registrado → **Simular notificación**.

Seleccioná tipo `payment` y pasá el ID de un pago existente.

**Qué verificar:** en los logs de tu backend, deberías ver algo como:
```
INFO  handling MP webhook  type=payment
INFO  webhook processed successfully  payment_id=1  status=approved
```

> **¿Por qué simular?** En local, MP no puede llegarte sin ngrok. La función "Simular notificación" del panel es la forma más rápida de probar sin configurar ngrok.

### 4.4 — Verificar estado

Corré `03_consultar_estado.yml` con el ID del pago que procesaste.

**Qué verificar:** el `status` debería estar actualizado (approved, rejected, o pending según el resultado del webhook).

---

## Paso 5: Verificar en la DB

Abrí la consola de Supabase (testing) y revisá la tabla `payments`:

```sql
SELECT id, concept, amount, status, status_detail, payment_id, payer_email, created_at
FROM payments
ORDER BY created_at DESC
LIMIT 10;
```

**Qué buscar:**
- `status` = `"approved"` si el pago fue exitoso.
- `payment_id` = el ID que MP asignó al pago (número).
- `raw_response` = la respuesta completa de MP (JSON).
- `payer_email` = el email del test user que usaste.

---

## Troubleshooting

| Error | Causa | Solución |
|---|---|---|
| `401 Unauthorized` en MP | Access Token de producción o vencido | Usá token de sandbox (`TEST-...`), verificá que no esté vencido |
| Webhook no llega | URL incorrecta o ngrok apagado | Verificá `MERCADOPAGO_WEBHOOK_URL`, reiniciá ngrok |
| Pago queda `pending` | Token de tarjeta inválido o test user incorrecto | Verificá el email del test user y los datos de la tarjeta |
| `payment not found` en webhook | El pago no se creó localmente | Verificá que `POST /payments` fue exitoso antes |
| `CORS error` | Frontend no está en `CORS_ALLOWED_ORIGINS` | Agregá el dominio del frontend en la config CORS |
| `invalid token` | Token de auth vencido o mal copiado | Corré `00_login` de nuevo y copiá el token completo |

---

## Tarjetas de test (sandbox)

| Tarjeta | Tipo | Resultado esperado |
|---|---|---|
| `5483 9281 6457 4623` | Visa | Aprobado |
| `5361 9568 0611 7557` | Mastercard | Aprobado |
| `4509 9535 6623 3704` | Visa | Rechazado |

**CVV:** cualquier 3 dígitos (ej: `123`)
**Fecha:** cualquier fecha futura (ej: `11/25`)
**DNI:** cualquier número (ej: `12345678`)

---

## Variables de entorno para Render

Cuando vayas a deployear, agregá estas env vars en el dashboard de Render:

| Variable | Valor |
|---|---|
| `MERCADOPAGO_ACCESS_TOKEN` | Access Token de homologación |
| `MERCADOPAGO_PUBLIC_KEY` | Public Key de homologación |
| `MERCADOPAGO_WEBHOOK_SECRET` | Secret del webhook |
| `MERCADOPAGO_WEBHOOK_URL` | `https://paceron-backend-develop.onrender.com/api/v1/payments/webhook` |
| `MERCADOPAGO_CURRENCY_ID` | `ARS` |
