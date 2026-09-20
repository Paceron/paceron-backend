# Reproducción del bug «cambio de tier paga pero no sube» — paso a paso (API)

Fecha: 2026-09-08
Instancia: backend de testing (`https://paceron-backend-as9c.onrender.com`)
Usuario usado: `pepa@lota.com` / `Abcd-12345` → `user_id: 3`, rol `entrenador` (`role_id: 2`).

> Nota: el token JWT de cada paso se obtuvo con el login del Paso 1 y se reusó en el header `Authorization: Bearer <token>` (no se replica la respuesta completa del login en cada paso).

Contexto del estado inicial (consultado con probe SQL a la DB de testing):
- `user_roles`: user 3 rol 2 → tier 3 `base_entrenador` (gratis), sin sub vigente.

Tiers del rol entrenador (de `GET /api/v1/tiers?role_id=2`):

| id | name | hierarchy | payment | amount |
|---|---|---|---|---|
| 3 | base_entrenador | 1 | no | 0 |
| 5 | medium_entrenador | 2 | sí | 300000 |
| 2 | premium_entrenador | 3 | sí | 200000 |

El flujo reportado a reproducir: base → tier 2 (funcionó) → tier 3 (pagó pero no subió).

---

## Paso 0 — Login y sanity check de estado

```bash
curl -s -X POST https://paceron-backend-as9c.onrender.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"pepa@lota.com","password":"Abcd-12345"}'
```

Respuesta relevante: `access_token: eyJhbGci...` (JWT), `user_id: 3`.

```bash
curl -s "https://paceron-backend-as9c.onrender.com/api/v1/users/3/subscriptions/current?role_id=2" \
  -H "Authorization: Bearer <token>"
```

Respuesta:
```json
{"next_due_date":null,"blocked_date":null,"tier":{"id":3,"name":"base_entrenador","hierarchy":1,"payment_required":false},"role":{"id":2,"name":"entrenador"}}
```

```bash
curl -s "https://paceron-backend-as9c.onrender.com/api/v1/tiers?role_id=2" -H "Authorization: Bearer <token>"
```

Respuesta: lista completa de tiers del rol (tabla de arriba).

---

## Paso 1 — Cambio base → medium (tier 5) — esta parte SÍ funciona según el reporte

```bash
curl -s -X PUT "https://paceron-backend-as9c.onrender.com/api/v1/users/3/roles/2/tier" \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"tier_id":5}'
```

Respuesta:
```json
{"subscription_id":8,"subscription_status":"first_payment_pending","installment_id":15,"installment_number":1,"installment_amount":300000,"next_due_date":null,"blocked_date":null,"paid_installments":0,"tier":{"id":5,"name":"medium_entrenador","hierarchy":2,"payment_required":true},"role":{"id":2,"name":"entrenador"},"mercadopago":{"public_key":"TEST-9a1e5d38-929e-45f8-ad90-01ca7887fe82"}}
```

### Paso 1.1 — Crear preferencia (cuota 15, $300000)

```bash
curl -s -X POST "https://paceron-backend-as9c.onrender.com/api/v1/payments/preference" \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"concept":"subscription","description":"Cuota de suscripcion de tier","items":[{"title":"Cuota mensual tier","quantity":1,"unit_price":300000}],"installment_id":15}'
```

Respuesta relevante: `preference_id: 40671376-5ed147b2-0694-4901-a8eb-c63442a3e0df`, `public_key: TEST-9a1e5d38-...`.

### Paso 1.2 — Tokenizar tarjeta de prueba

```bash
curl -s -X POST "https://paceron-backend-as9c.onrender.com/api/v1/payments/test-card-token" \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"card_number":"5031755734530604","expiration_month":"11","expiration_year":"2030","security_code":"123","cardholder_name":"APRO Test User","identification_type":"DNI","identification_number":"12345678"}'
```

Respuesta relevante: `card_token: 02ec01c1b0851316b51f161c94da6739`.

### Paso 1.3 — Procesar pago ($300000)

```bash
curl -s -X POST "https://paceron-backend-as9c.onrender.com/api/v1/payments" \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"token":"02ec01c1b0851316b51f161c94da6739","transaction_amount":300000,"payment_method_id":"master","installments":1,"payer_email":"comprador.e2e.2026@example.com","preference_id":"40671376-5ed147b2-0694-4901-a8eb-c63442a3e0df","installment_id":15}'
```

Respuesta:
```json
{"id":91,"preference_id":"40671376-5ed147b2-0694-4901-a8eb-c63442a3e0df","payment_id":"1328097670","external_reference":"","concept":"order","description":"","amount":300000,"currency_id":"ARS","status":"approved","status_detail":"accredited","payment_method_id":"master","installments":1,"payer_email":"comprador.e2e.2026@example.com","created_at":"2026-09-08T00:56:15Z"}
```

### Paso 1.4 — Verificar estado (≈8s después, esperando el webhook)

```bash
curl -s "https://paceron-backend-as9c.onrender.com/api/v1/users/3/subscriptions/current?role_id=2" \
  -H "Authorization: Bearer <token>"
```

Respuesta:
```json
{"subscription_id":8,"subscription_status":"active","installment_id":16,"installment_number":2,"installment_amount":300000,"next_due_date":"2026-10-08T00:55:54.962767Z","blocked_date":"2026-10-15T00:55:54.962767Z","paid_installments":1,"tier":{"id":5,"name":"medium_entrenador","hierarchy":2,"payment_required":true},"role":{"id":2,"name":"entrenador"},"mercadopago":{"public_key":"TEST-9a1e5d38-929e-45f8-ad90-01ca7887fe82"}}
```

✅ Tier subió a `medium_entrenador` (5), sub 8 activa, cuota #1 (15) pagada, cuota #2 (16) pendiente.

---

## Paso 2 — Cambio medium → premium (tier 2) — el caso reportado

```bash
curl -s -X PUT "https://paceron-backend-as9c.onrender.com/api/v1/users/3/roles/2/tier" \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"tier_id":2}'
```

Respuesta:
```json
{"subscription_id":9,"subscription_status":"first_payment_pending","installment_id":17,"installment_number":1,"installment_amount":200000,"next_due_date":null,"blocked_date":null,"paid_installments":0,"tier":{"id":2,"name":"premium_entrenador","hierarchy":3,"payment_required":true},"role":{"id":2,"name":"entrenador"},"mercadopago":{"public_key":"TEST-9a1e5d38-929e-45f8-ad90-01ca7887fe82"}}
```

### Paso 2.1 — Crear preferencia (cuota 17, $200000)

```bash
curl -s -X POST "https://paceron-backend-as9c.onrender.com/api/v1/payments/preference" \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"concept":"subscription","description":"Cuota de suscripcion de tier","items":[{"title":"Cuota mensual tier","quantity":1,"unit_price":200000}],"installment_id":17}'
```

Respuesta relevante: `preference_id: 40671376-30b514eb-3eac-45ed-9c6f-406eec19c323`.

> ⚠️ En un primer intento se pasó el JSON completo de la preferencia como `preference_id` (error de armado del request), lo que dio `HTTP 400 {"message":"Cuerpo de solicitud inválido"}` — no es un fallo del backend, fue del lado de la prueba. Se repitió correctamente.

### Paso 2.2 — Tokenizar tarjeta de prueba

```bash
curl -s -X POST "https://paceron-backend-as9c.onrender.com/api/v1/payments/test-card-token" \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"card_number":"5031755734530604","expiration_month":"11","expiration_year":"2030","security_code":"123","cardholder_name":"APRO Test User","identification_type":"DNI","identification_number":"12345678"}'
```

Respuesta relevante: `card_token: 77520b14e4f5174cb1c8f0be3892fab5`.

### Paso 2.3 — Procesar pago ($200000)

```bash
curl -s -X POST "https://paceron-backend-as9c.onrender.com/api/v1/payments" \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"token":"77520b14e4f5174cb1c8f0be3892fab5","transaction_amount":200000,"payment_method_id":"master","installments":1,"payer_email":"comprador.e2e.2026@example.com","preference_id":"40671376-30b514eb-3eac-45ed-9c6f-406eec19c323","installment_id":17}'
```

Respuesta:
```json
{"id":94,"preference_id":"40671376-30b514eb-3eac-45ed-9c6f-406eec19c323","payment_id":"1328095960","external_reference":"","concept":"order","description":"","amount":200000,"currency_id":"ARS","status":"approved","status_detail":"accredited","payment_method_id":"master","installments":1,"payer_email":"comprador.e2e.2026@example.com","created_at":"2026-09-08T00:57:18Z"}
```

### Paso 2.4 — Verificar estado (≈10s después)

```bash
curl -s "https://paceron-backend-as9c.onrender.com/api/v1/users/3/subscriptions/current?role_id=2" \
  -H "Authorization: Bearer <token>"
```

Respuesta:
```json
{"subscription_id":9,"subscription_status":"active","installment_id":18,"installment_number":2,"installment_amount":200000,"next_due_date":"2026-10-08T00:56:45.84075Z","blocked_date":"2026-10-15T00:56:45.84075Z","paid_installments":1,"tier":{"id":2,"name":"premium_entrenador","hierarchy":3,"payment_required":true},"role":{"id":2,"name":"entrenador"},"mercadopago":{"public_key":"TEST-9a1e5d38-929e-45f8-ad90-01ca7887fe82"}}
```

### Estado DB tras el Paso 2 (probe SQL)

```sql
-- user_roles: user 3 rol 2 ahora tier_id=2, active
-- subs: sub 8 (medium) ended; sub 9 (premium) active, paid_installments=1
-- installments:
--   15 (sub 8, #1) paid  -> pago 91
--   16 (sub 8, #2) pending
--   17 (sub 9, #1) paid  -> pago 94
--   18 (sub 9, #2) pending
```

✅ **El flujo "correcto" (pagar la cuota #1 que devuelve el ChangeTier) SÍ sube el tier**: quedó en `premium_entrenador` (2). El bug NO se reproduce siguiendo la API al pie de la letra.

---

## Hallazgo / hipótesis a seguir

La reproducción correcta no falla. La hipótesis restante es el **pago de la cuota vieja**: al cambiar de tier, la sub anterior queda `ended` pero **su cuota pendiente (la #2) sigue viva**. En el estado actual conviven cuotas pendientes de subs distintas:

- `installments` 16 (sub 8, medium, #2, 300000) — pendiente, sub `ended`
- `installments` 18 (sub 9, premium, #2, 200000) — pendiente, sub `active`

Si el frontend manda el `installment_id` de la cuota vieja (que tenía de la sub anterior) en vez del que devuelve `ChangeTier`, `applyApprovedInstallment` marca paid una cuota con `installment_number != 1` → **no ejecuta `UpdateTier`** → el pago se acredita pero el tier no sube. Eso matchea el reporte.

Pendiente (siguiente iteración): reproducir pagando la cuota vieja `18` (o `16`) y verificar que el tier permanece sin actualizarse (pago `approved`, tier sin cambio). Los logs nuevos de diagnóstico (`applyApprovedInstallment confirming` / `updating tier`) en Render permiten confirmarlo por la traza.