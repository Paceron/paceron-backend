# Design

## Context

Ver `proposal.md` para la motivación y `specs/dashboard-objective-metrics/spec.md` para el contrato observable.

`BuildSummary` construye un `setMetrics` compartido también por timeseries, leaderboard y comparaciones. `bestTimeMs` se calcula sobre filas válidas; la tolerancia se aplica después al calcular `complianceRate`. `sumTargetDistanceM` se calcula sobre filas válidas con objetivo y alimenta `deltaPct`, por lo que cambiar su población modificaría una métrica existente. Cada `ScopedSeries` expone el objetivo congelado de su instancia con `Targets()`, y `metTarget` evalúa el cumplimiento usando la tolerancia recibida.

## Goals / Non-Goals

**Goals:**

- Calcular los nuevos acumulados únicamente para `SummaryTotals`, sin cambiar los resultados de los agregados compartidos por otros endpoints.
- Reutilizar los helpers de objetivo, validez y cumplimiento existentes, incluyendo la tolerancia normalizada que recibe `BuildSummary`.
- Representar de forma distinta la ausencia de objetivos (`null`) y un total logrado legítimo de cero.

**Non-Goals:**

- Generalizar los nuevos campos a timeseries, leaderboard, comparaciones o períodos.
- Alterar el cálculo de `sumTargetDistanceM`, `deltaPct`, `bestTimeMs`, `complianceRate` o bandas.
- Añadir una nueva regla de cumplimiento o derivar un porcentaje en el backend.

## Decisions

### 1. Calcular las métricas nuevas en el armado del resumen

`BuildSummary` calculará `best_time_met_ms` y los acumulados de distancia objetivo/lograda a partir de las filas de su scope y las colocará en `SummaryTotals`. El cálculo del mejor tiempo usará `metTarget` con la tolerancia recibida. El conjunto candidato será el de filas con `met_target=true` y tiempo positivo; no dependerá de la banda ni redefinirá la validez global de los agregados.

**Alternativas consideradas:** ampliar `newSetMetrics` con campos para todos los endpoints o cambiar su firma para que reciba tolerancia. Se elige el cálculo en `BuildSummary` para mantener el nuevo contrato acotado al resumen y no hacer trabajo adicional en los otros agregados. Para el mejor tiempo también se descarta usar `band`, ya que el contrato depende de `met_target` y la banda es una clasificación distinta.

### 2. Mantener un acumulador de distancia separado del usado por `deltaPct`

El conjunto de filas para los acumulados de distancia se determina por el tipo (`objectiveAxisForKind` debe incluir distancia) y un objetivo congelado positivo. `target_distance_m` suma esos objetivos en todas las filas del scope, sin filtrar por `Valid()`. `achieved_target_distance_m` suma la distancia lograda positiva solo cuando la fila es válida; las demás aportan cero. Ambos valores permanecen `nil` si no hay filas con objetivo de distancia.

`sumTargetDistanceM` y `sumAchievedTargetedDistanceM` conservan su población actual, porque alimentan `deltaPct`. No se reutilizará ni se ampliará esa suma para producir el nuevo par.

**Alternativas consideradas:** reutilizar el acumulador actual o excluir las filas inválidas de ambos lados. Reutilizarlo alteraría `deltaPct`; excluir las filas inválidas de los dos lados permitiría que una serie no medida desaparezca del denominador. Se conserva el comportamiento existente y se añade una suma específica con el denominador acordado.

### 3. Usar punteros en el DTO para conservar la nulabilidad

`SummaryTotals` incluirá `BestTimeMetMs *int64`, `TargetDistanceM *float64` y `AchievedTargetDistanceM *float64`, con los nombres JSON establecidos en la spec y sin `omitempty`. Así, la respuesta serializa siempre las claves: `null` representa ausencia de un valor calculable y `0` representa un total calculado legítimamente. Se regenerará Swagger con el comando documentado por el repositorio.

**Alternativas consideradas:** usar escalares con cero como valor inicial o campos con `omitempty`. Los escalares confundirían ausencia con un total cero; `omitempty` eliminaría claves en vez de preservar el contrato nullable.

## Risks / Trade-offs

- [El filtro de tipos podría incluir distancias que no son objetivos para ese ejercicio] → determinar el eje con `objectiveAxisForKind` y exigir un objetivo congelado positivo.
- [Un cambio accidental en la población compartida podría alterar `deltaPct` u otros endpoints] → mantener intactos los acumuladores existentes y limitar los nuevos al armado de `SummaryTotals`.
- [Un consumidor podría interpretar `null` como cero o ignorar un logrado cero con objetivo positivo] → conservar tipos nullable en el DTO y cubrir los tres estados en las pruebas de agregado y serialización.
- [La tolerancia podría diferir entre cumplimiento visible y mejor tiempo condicionado] → pasar a `metTarget` la misma tolerancia normalizada que recibe `BuildSummary` y probar límites distintos.

## Migration Plan

No se requiere migración de base de datos ni conversión de datos: los objetivos se leen de las instancias congeladas existentes. El cambio de API es aditivo; se despliega el backend con los tres campos nullable y se coordina el cliente para que tolere su ausencia durante una actualización mixta y `null` cuando el scope no tenga un valor calculable. El rollback consiste en volver a desplegar la versión anterior del backend; los campos previos conservan su semántica.
