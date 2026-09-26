# Diseño: Historial de pagos y cobros del entrenador

## Context

Los pagos viven en `payments` y se vinculan a una cuota (`installments`) por
`payments.installment_id`. La cuota apunta a una suscripción de tier
(`subscription_id`) o a un equipo (`team_id`), nunca a los dos.

Relevado en el código antes de diseñar:

- **`payments.user_id` no es confiable.** `ProcessPayment` no lo setea y
  `CreatePreference` guarda el `seller_id` que manda el cliente. El dueño real de
  una cuota es `installments.user_id`.
- **El concepto de los pagos de tier suele quedar mal.** El frontend no manda
  `concept` en `POST /payments`, así que esas filas quedan con `order`.
- **`CreatePreference` deja una fila `pending` sin `payment_id`.** Después
  `ProcessPayment` busca esa fila por `external_reference` usando el
  `preference_id`, nunca la encuentra y crea otra. Por cada cobro real queda una
  fila que nunca llegó a Mercado Pago.
- **El neto solo aparece si el pago pasó por el webhook.** `ProcessPayment` guarda
  en `raw_response` un `PaymentResult` sin `transaction_details`. El webhook sí
  guarda el `payment.Response` completo del SDK, que trae
  `transaction_details.net_received_amount`.
- **`marketplace_fee` es ficticia.** Se calcula y se guarda, pero nunca viaja a
  Mercado Pago (`application_fee` no se setea).

## Goals / Non-Goals

**Goals:**
- Listar los cobros de equipo del usuario autenticado y sus pagos de tier.
- Resumir los cobros por mes y por equipo.
- Mostrar el neto solo cuando es real.

**Non-Goals:**
- Corregir el webhook, las filas sin `payment_id` o el envío de
  `application_fee`. Se registran como deuda técnica.

## Decisions

### D1 — DAO nuevo en vez de ampliar `PaymentDaoInterface`
`PaymentHistoryDao` es de solo lectura y vive aparte. Ampliar
`PaymentDaoInterface` obligaría a tocar el `mockPaymentDao` de
`payment_service_test.go` y todos sus usos, sin ganar nada.

### D2 — Service propio, sin delegate
`PaymentHistoryService` depende solo de `PaymentHistoryDao`. No combina services,
así que no hace falta un delegate.

### D3 — Controller propio
`PaymentHistoryController` no toca la firma de `NewPaymentController`.

### D4 — Sin 403 por rol
Los datos quedan acotados por el token: `payments.seller_user_id = me` para los
cobros e `installments.user_id = me` para los pagos de tier. Un ex entrenador
tiene que poder ver su historial, y hay menos ramas para testear.

### D5 — Neto real, sin exponer `marketplace_fee`
El neto sale de `raw_response->'transaction_details'->>'net_received_amount'`
solo si el pago está `approved`, el valor es numérico y es mayor que 0. En
cualquier otro caso es `null`. Un pago pendiente que pasó por el webhook trae el
neto en 0, por eso se descarta.

### D6 — Meses en hora argentina, sobre `created_at`
No hay columna de fecha de aprobación. Los meses se cortan con
`time.FixedZone("ART", -3*3600)`: Argentina no tiene horario de verano desde
2009, y así no se depende de que el container tenga `tzdata`. Consecuencia
aceptada: un pago iniciado el 31 a las 23:59 y aprobado el 1 cae en el mes
anterior.

### D7 — Se excluyen las filas sin `payment_id`
Todas las consultas filtran `COALESCE(p.payment_id, '') <> ''`. Sin este filtro,
los pendientes siempre dan de más.

### D8 — Pendientes y rechazados por cuota
Se cuenta sobre el **último intento** de cada cuota dentro de la ventana
(`created_at DESC, id DESC`). Si la cuota tiene algún intento aprobado, no
cuenta. Así un rechazo seguido de un pago aprobado no aparece como problema.

### D9 — Agregación en Go
El resumen se calcula en el service sobre un único SELECT acotado a la ventana
de meses. Con 50 corredores en 6 meses son unas 300 filas. La lógica queda
testeable con mocks. Si un entrenador supera las ~5.000 filas en la ventana, se
migra a un `GROUP BY` en SQL.

### D10 — Paginación de 20 con `has_more`
Parámetro `page` y tamaño fijo 20. Se piden `pageSize+1` filas para derivar
`has_more` sin un `COUNT(*)`, igual que `teamDao.SearchPublic`.

### D11 — Cobros por `seller_user_id`
Se filtra por quien recibió el dinero, no por `teams.owner_id`, que puede
cambiar si el equipo cambia de dueño.

## Contrato

Estados de Mercado Pago agrupados en `status_group`:

| Grupo | Estados |
|---|---|
| `approved` | `approved` |
| `pending` | `pending`, `in_process`, `authorized` |
| `rejected` | `rejected`, `cancelled` |
| `refunded` | `refunded`, `charged_back` |

Solo `approved` suma como cobrado.

### `GET /api/v1/payments/received?page=&team_id=&status=`

```json
{
  "payments": [
    {
      "id": 812,
      "mp_payment_id": "1319998877",
      "status": "approved",
      "status_group": "approved",
      "status_detail": "accredited",
      "gross_amount": 15000,
      "net_amount": 14101.5,
      "currency_id": "ARS",
      "payment_method_id": "visa",
      "created_at": "2026-09-14T16:22:05Z",
      "installment_id": 301,
      "installment_number": 3,
      "team": { "id": 12, "name": "Runners del Parque" },
      "payer": { "id": 45, "name": "Lucía", "surname": "Gómez", "email": "lucia.gomez@mail.com" }
    }
  ],
  "has_more": true
}
```

### `GET /api/v1/payments/mine?page=&role=`

```json
{
  "payments": [
    {
      "id": 790,
      "mp_payment_id": "1319990011",
      "status": "approved",
      "status_group": "approved",
      "status_detail": "accredited",
      "amount": 9999,
      "currency_id": "ARS",
      "payment_method_id": "master",
      "created_at": "2026-09-02T12:10:00Z",
      "installment_id": 210,
      "installment_number": 2,
      "due_date": "2026-09-05T03:00:00Z",
      "subscription_id": 55,
      "tier": { "id": 4, "name": "Premium_entrenador", "role_name": "entrenador" }
    }
  ],
  "has_more": false
}
```

### `GET /api/v1/payments/received/summary?months=`

`months` entre 2 y 12, 6 por defecto. `monthly` siempre trae `months` meses
seguidos y el último es el mes actual.

```json
{
  "currency_id": "ARS",
  "months": 6,
  "monthly": [
    { "month": "2026-09", "gross_amount": 111000, "net_amount": 31000.5, "approved_count": 7, "net_known_count": 2 }
  ],
  "by_team": [
    { "team_id": 12, "team_name": "Runners del Parque", "gross_amount": 405000, "net_amount": 250000, "approved_count": 27, "net_known_count": 17, "pending_count": 1, "rejected_count": 0 }
  ],
  "pending_count": 2,
  "rejected_count": 1,
  "generated_at": "2026-09-26T18:00:00Z"
}
```

## Risks / Trade-offs

- **El neto va a faltar en la mayoría de los cobros**, porque el webhook lee los
  pagos de equipo con el token de la plataforma. Se muestra como `null` y queda
  registrado como deuda.
- **`created_at` no es la fecha de aprobación** (ver D6).
- **Huso fijo en −03:00.** Si Argentina volviera a tener horario de verano, los
  bordes de mes se correrían una hora.
- **Coverage al límite (85,2% con umbral 85).** Cada tarea lleva sus tests en el
  mismo commit.

## Migration Plan

- Índice nuevo por AutoMigrate. No hay migración de datos.
- Rollback: revertir los commits; no hay cambios de contrato en endpoints
  existentes.

## Open Questions

- ¿Conviene guardar `approved_at` desde el webhook para agrupar por fecha de
  aprobación? Queda para otro change.
