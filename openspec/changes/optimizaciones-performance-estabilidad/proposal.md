# Optimizaciones de performance y estabilidad

## Por qué

La auditoría de performance/seguridad de `develop` detectó tres fallos críticos de estabilidad/seguridad y varios N+1 en caminos de apertura de la app. El objetivo es reducir latencia y riesgo operativo sin cambiar contratos HTTP, DTOs, mecanismos de autenticación ni el comportamiento funcional esperado.

## Qué

Etapas acotadas, todas verificables con tests:

1. **Estabilidad crítica:** eliminar panics por `*gin.Context` nil en pushes y por logging con `ctx.Copy()`; recuperar panics en la entrega WS.
2. **Seguridad de escritura:** validar acceso a la instancia en creación de feedback y runner sessions; validar ejercicio/equipo coherentes; cerrar el IDOR de `POST /payments` con `preference_id`.
3. **Pagos:** no aceptar montos de cuota distintos del monto persistido en `ProcessPayment`/`CreatePreference`; no aplicar cuotas si el webhook aprueba un monto menor.
4. **Lecturas N+1:** permisos, join requests, dashboard scope/atletas y reutilización de `openSessions`.
5. **Calendario/CPU:** detalle de sesión sin doble `FindByID`, escritura de un día con helpers batch, `Shift` batch, eliminar copia+sort redundante en `peak_speed`, regex y base de media URL cacheadas.
6. **Gamificación/push:** unificar el cálculo de rachas post-finish y batchear el fan-out de push del cierre de competencia.
7. **DB/config:** índice `workout_feedback.feedback_owner_user_id` y pool de conexiones configurable por env con defaults actuales.

## Alcance

Solo backend. Se preservan shapes de respuesta, códigos HTTP esperados y flujos existentes. Los únicos cambios observables son:
- 403/400 en escrituras cross-tenant que antes no se validaban.
- 404 en `POST /payments` con `preference_id` ajeno.
- 400 cuando el monto de una cuota no coincide.
- push del cierre de competencia con menor latencia (mismo contenido).

## No-goals

- No se agregan límites o paginación a búsquedas que hoy devuelven todo.
- No se agrega gzip ni `PureJSON`.
- No se cambia la visibilidad de equipos ni la PII de `GET /auth/user` (decisión de producto aparte).
- No se propaga `WithContext` a todos los DAOs en esta ronda.
- No se modifica el lock de `CreateAttemptSession` ni el envío de email sincrónico.
- No se toca la lógica de tiers del corredor (change `gap-42-tiers-corredor`).

## Capabilities

- `api-performance` (interna; sin cambio de contrato).
- `api-security` (endurecimiento de autorización y montos).
