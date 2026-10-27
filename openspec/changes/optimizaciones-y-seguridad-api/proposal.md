# Propuesta: optimizaciones-y-seguridad-api

## Why (Por qué)

Ronda de revisión de performance y seguridad sobre áreas de la API nunca auditadas (pagos, auth, catálogos, push, MP client). La app funciona pero no es rápida: hay N+1 reales en listados que el frontend abre todo el tiempo, joins sin índices sobre las tablas que más crecen, y brechas de seguridad concretas (webhook sin firma validada, OAuth state forjable, IDOR en pagos, sesiones que sobreviven al cambio de contraseña). Todo se resuelve **sin cambiar shapes ni contratos observables para el frontend**.

Decisiones de producto tomadas con el usuario (2026-10-16):
- `/auth/user` **queda público con su shape completo** — no se restringe.
- CRUD de roles/permissions/tiers y `platform-settings/marketplace-fee` **no se tocan** — esperan al módulo de gestión administrativa (rol admin formal).
- `POST /payments/test-card-token` **se mantiene** (sin UI propia en frontend, puede servir).
- `GET /groups` sin team_id **se mantiene** — limpieza futura en trabajo específico.

## What (Qué)

**Stage 1 — Optimizaciones (cero cambios de API):**
- P1: `GET /sessions` pasa de 1+N a 2 queries (batch `FindBySessions` ya existente, agrupado en memoria).
- P2: `GET /training-plans` pasa de 1+N a 2 queries (nuevo batch `FindByPlanIDs`).
- P3: mensajería de sesión — tope de destinatarios (100) + chequeo de acceso en batch en vez de 1 query por destinatario.
- P5: `ProcessPayment` deja de releer la cuota (reutiliza la ya validada).
- P6: MP client crudo gana timeouts (reemplazo de `&http.Client{}` sin Timeout por la infra `httpclient` del repo o `http.Client{Timeout: 10s}` compartido).
- P7: push de Expo batchea tokens en un solo POST por usuario (Expo lo soporta) en vez de 1 HTTP por token.
- P8: roles del login/refresh en 1 query (`IN`) en vez de 1 por rol.
- P9: validación de catálogo en create/clone de sesiones y planes en batch (`FindByIDs`) en vez de 1 por item.
- 13 índices gorm (ver design D7) sobre tablas calientes nunca indexadas.

**Stage 2 — Seguridad:**
- S1: webhook de Mercado Pago valida la firma `X-Signature` (ts,v1) con `ValidateWebhookSignature` (ya implementado y probado, sin llamador) — 401 si inválida; sin secret configurado mantiene comportamiento actual + warning.
- S2: OAuth callback de MP Connect — `state` firmado con HMAC (userID+ts+target) — el state forjable que vincula la cuenta MP del atacante a la víctima deja de existir.
- S5: IDOR en `GET /payments/:id` y `GET /payments/mp/:id` — guard de ownership (payer o seller), 404 en otro caso.
- S6: cambio/reset de contraseña revoca los refresh tokens del usuario.
- S7: webhook de deauth de MP Connect valida firma con el mismo mecanismo (gradual igual que S1).
- S9: MaxBytesReader global en middleware (tope por request, cubre JSON y multipart).
- S10: rate limiter por IP en las rutas públicas (login/register/forgot/reset/refresh/auth/user) — en memoria.

## Why 2 stages

Stage 1 es mecánico y sin riesgo observable; stage 2 cambia comportamiento de endpoints de integración externa (MP) e introduce un middleware nuevo. Revisarlos separados mantiene cada review acotada.

## Capabilities

- Delta ADDED sobre `payments` (ownership guard + validación de firma de webhook).
- Nueva capability `auth-security` (revocación de sesiones, rate limit, límite de body).
- Delta ADDED sobre `mp-connect` (state HMAC + firma de deauth).

## No-goals

- Rol admin formal / guards de roles-tiers-fee (S3/S4) — módulo de gestión administrativa futuro.
- Restringir `/auth/user` (S8) — decisión de producto: queda público.
- Eliminar/restringir `test-card-token` (S12) — se conserva.
- Scope de `GET /groups` (S11) — limpieza futura.
- Rate limiting global sobre todas las rutas autenticadas (solo públicas).
- Persistencia del rate limiter / límites compartidos entre réplicas (single instance en Render).
