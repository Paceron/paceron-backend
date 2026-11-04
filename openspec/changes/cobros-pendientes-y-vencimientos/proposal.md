# Propuesta: Cobros pendientes, vencimientos y comisión en todos los cobros

## Why

La vista de cobros y pagos del frontend muestra datos incompletos:

- **Cobros sin comisión.** 27 de 104 cobros corredor → entrenador de develop no tenían `marketplace_fee`. Sin comisión el neto no existe, así que el entrenador veía "Neto no disponible". Tenían dos orígenes: 18 eran filas cargadas a mano sin comisión, y 9 se crearon el 2026-10-06 con `marketplace_fee_percent = 0.00001`. Con ese porcentaje la comisión redondea a 0, y el código guarda `NULL` cuando da 0. Los datos de testing ya se corrigieron con un script (5%).
- **El neto depende de Mercado Pago.** Hoy el neto es `transaction_details.net_received_amount`, que solo llega por webhook y además descuenta el costo de procesamiento de MP. El equipo decidió que el neto de un cobro sea **bruto − comisión de Paceron**, siempre disponible.
- **No hay forma de ver lo que falta cobrar.** Las cuotas se crean de a una: la N+1 recién aparece cuando se paga la N. Si un corredor deja de pagar, los meses siguientes no existen en ninguna tabla, y el entrenador no puede ver cuánto le deben, ni qué vence este mes, ni qué ya venció.
- **El que paga no ve sus próximos vencimientos**, ni de la cuota del equipo ni de su tier.

## Objetivo

1. Que ningún cobro corredor → entrenador quede sin comisión, y que el neto sea bruto − comisión.
2. Que el entrenador vea sus cobros pendientes por mes, lo que vence en el mes y lo que ya venció en el mes.
3. Que cualquier usuario vea el próximo vencimiento de cada cuota de equipo que paga y de cada suscripción de tier.

## Alcance

- **Código de pagos:** un cobro de equipo guarda siempre su comisión, también cuando redondea a 0, y un pago de equipo sin `installment_id` se rechaza.
- **Neto:** el historial de cobros y el resumen calculan el neto como `amount − marketplace_fee`. Los cobros exponen además la comisión.
- **Obligaciones calculadas:** un cálculo puro que deriva las cuotas esperadas de cada membresía y suscripción a partir del alta, el monto y las cuotas pagadas. No se agrega ninguna tabla.
- **Resumen de cobros:** suma pendientes por mes y dos bloques, a vencer y vencidos del mes.
- **Endpoint nuevo:** `GET /api/v1/payments/upcoming`, con los próximos vencimientos del usuario.
- DAO, service, controller, swagger, tests y documentación.

## No alcance

- Materializar las obligaciones en una tabla, o generar cuotas de meses no pagados.
- Mostrar los cobros de tier al entrenador: son ingresos de Paceron. El entrenador solo ve sus propios pagos de tier en el historial.
- Una migración de datos en el código: los datos se corrigieron con un script en testing, por decisión del equipo.
- Cambiar el porcentaje de comisión o cómo se configura.

## Métrica de éxito

- 0 cobros `team_subscription` con `marketplace_fee` NULL después del cambio.
- El resumen de develop muestra el neto de todos los cobros aprobados, sin "neto parcial".
- Para la membresía del equipo 20, usuario 4 (alta 2026-09-06, 1 cuota pagada), el entrenador ve pendiente en octubre la cuota que venció el 2026-10-06, y a vencer este mes (noviembre) la del 2026-11-06.
