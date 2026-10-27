# Spec Delta

## Purpose

Exponer una comparación segura y agregada de dos ejecuciones reales de ejercicios del dashboard, reutilizando las métricas del dominio y sin trasladar cálculos al cliente.

## ADDED Requirements

### Requirement: La API SHALL exponer las dos referencias de sesión y ejercicio

El backend SHALL exponer `POST /api/v1/dashboard/comparisons/sessions` con `team_id`, fechas ISO `from`/`to` y lados `a`/`b`. Cada lado SHALL incluir `group_id`, `athlete_user_id`, `session_instance_id` y `exercise_id` de catálogo.

#### Scenario: Request completo con dos lados válidos

- **WHEN** un usuario autenticado envía dos referencias válidas
- **THEN** la API responde `200` con ambos resúmenes y su delta

### Requirement: La API SHALL validar referencias y rechazar resultados parciales

Los IDs SHALL ser positivos, el rango SHALL ser ISO inclusivo y no superar dos años, y campos desconocidos SHALL rechazarse. El endpoint SHALL validar ambas referencias antes de responder; una falla no SHALL filtrar el otro lado.

#### Scenario: Body o fechas inválidos

- **WHEN** faltan campos, hay IDs/fechas inválidos o campos desconocidos
- **THEN** responde `400` sin consultar feedback

#### Scenario: Una referencia no coincide con la sesión

- **WHEN** cualquiera de los lados no corresponde a feedback elegible
- **THEN** rechaza la comparación completa sin exponer resultados parciales

### Requirement: El scope SHALL autorizar cada lado sin ampliar permisos

El endpoint SHALL autorizar ambos lados dentro del scope del mismo team. El DAO SHALL aplicar sus predicados antes de agregar y el resultado SHALL ser todo o nada.

Los grupos SHALL pertenecer al team y scope actuales. Entrenadores SHALL validar ambos atletas y su membresía en la fecha de cada sesión. Miembros SHALL enviar su propia identidad en ambos lados. Un ID de sesión no SHALL ser autorización independiente.

#### Scenario: Entrenador compara el mismo corredor

- **WHEN** ambos lados identifican al mismo atleta en grupos accesibles de un team propio
- **THEN** la API permite comparar las dos sesiones y valida ambas membresías en sus fechas respectivas

#### Scenario: Entrenador compara dos corredores

- **WHEN** A y B pertenecen a corredores distintos que estaban en los grupos respectivos en las fechas consultadas
- **THEN** la API devuelve ambos resultados dentro del team autorizado

#### Scenario: Atleta consulta sesiones propias

- **WHEN** los dos `athlete_user_id` coinciden con el usuario autenticado y los grupos/sesiones están dentro de su scope
- **THEN** la API permite la comparación

#### Scenario: Atleta intenta leer datos de otra persona

- **WHEN** cualquiera de los lados contiene un atleta distinto al usuario autenticado
- **THEN** la API responde `403` y no devuelve ningún resumen

#### Scenario: Una de dos referencias está fuera del scope

- **WHEN** A es válida y B apunta a un grupo, corredor o sesión ajeno
- **THEN** la API rechaza toda la request y no revela las métricas de A

### Requirement: Solo feedback elegible de ejercicios principales SHALL entrar al cálculo

Cada lado SHALL resolver el ejercicio a la instancia congelada por `source_exercise_id` (con fallback legacy del mismo owner) y filtrar `role = 'main'` en el join de sesión/instancia.

Solo SHALL entrar feedback no eliminado del atleta, sesión y rango referenciados. Se aceptan sesiones `finished`, `interrupted` o sin fila `runner_session`; `wip`, `warmup` y `cooldown` SHALL excluirse. Sin feedback ejecutado, la referencia SHALL ser inexistente/no ejecutada.

#### Scenario: Se conserva feedback histórico sin runner_session

- **WHEN** la sesión seleccionada tiene feedback y no tiene fila `runner_session`
- **THEN** las series elegibles se incluyen como histórico legacy

#### Scenario: Sesión aún en curso

- **WHEN** la sesión tiene estado `wip`
- **THEN** sus filas no entran al resumen ni a las series de comparación

#### Scenario: El ejercicio existe como calentamiento

- **WHEN** una instancia tiene feedback pero su vínculo de sesión declara `role = 'warmup'` o `cooldown`
- **THEN** no se considera una referencia comparable del dashboard

#### Scenario: El ejercicio legacy no tiene source_exercise_id

- **WHEN** el catálogo y la instancia legacy coinciden por nombre normalizado y pertenecen al mismo owner
- **THEN** la instancia puede resolverse para la comparación; no se descarta silenciosamente el histórico

### Requirement: Las métricas SHALL derivarse con las reglas del dominio dashboard

Cada lado SHALL devolver series registradas/válidas, distancia total válida, velocidad media ponderada por distancia y mejor/peor. El rendimiento SHALL reutilizar las reglas de validez y tiempo logrado del dominio dashboard.

El RPE SHALL promediar valores persistidos de 1–10 e informar `rpe_count`; su ausencia no SHALL alterar métricas de rendimiento. Los valores no calculables SHALL ser `null`. `series_count` cuenta filas elegibles y `valid_series` las filas válidas según el tipo.

#### Scenario: Sin valores de RPE

- **WHEN** las filas de una sesión no contienen RPE
- **THEN** `mean_rpe` es `null`, `rpe_count` es cero y las métricas de rendimiento no cambian

### Requirement: Los extremos SHALL respetar el tipo congelado del ejercicio

Mejor y peor SHALL usar la métrica definida por el tipo congelado del ejercicio. Las series inválidas SHALL excluirse y los empates SHALL resolverse determinísticamente por fecha, `set_number` y feedback ID.

#### Scenario: Mejor/peor de ritmo continuo

- **WHEN** el lado es `ritmo_continuo` con series válidas
- **THEN** mejor y peor se ordenan por tiempo logrado y cada extremo incluye la distancia realmente recorrida

#### Scenario: Mejor/peor de velocidad por distancia

- **WHEN** el lado es `velocidad_distancia`
- **THEN** mejor y peor se ordenan por `speed_kph` calculada desde distancia y tiempo, no por duración aislada

#### Scenario: Mejor/peor de velocidad por tiempo

- **WHEN** el lado es `velocidad_tiempo`
- **THEN** mejor y peor se ordenan por distancia lograda durante el tiempo objetivo

#### Scenario: Métricas ausentes

- **WHEN** no hay series válidas, velocidad calculable o valores RPE
- **THEN** los valores respectivos son `null`; los conteos permanecen explícitos y no se sustituyen por cero

### Requirement: La respuesta SHALL declarar el delta y proveer puntos para el gráfico

Cada lado SHALL incluir metadatos, conteos, extremos y puntos ordenados por `set_number` y feedback ID. Los puntos SHALL exponer tiempos, distancia, velocidad derivada y RPE.

El delta SHALL declarar dirección `b_minus_a`, velocidad y distancia como B−A, y porcentaje de velocidad `(B − A) / A × 100`. La respuesta SHALL declarar unidades y conservar `null` cuando falte una magnitud calculable.

#### Scenario: B mejora la velocidad

- **WHEN** ambos lados tienen velocidad y B supera A
- **THEN** `speed_delta_kph` y `speed_delta_pct` son positivos y la dirección es `b_minus_a`

#### Scenario: Delta no calculable

- **WHEN** falta la velocidad de A o B, o A no es positiva para el porcentaje relativo
- **THEN** el delta aplicable es `null`, no cero

#### Scenario: Orden determinista de los puntos

- **WHEN** dos series comparten `set_number`
- **THEN** el orden secundario por feedback ID hace estable la respuesta entre requests

### Requirement: Los errores SHALL mantener el contrato HTTP del dashboard

El endpoint SHALL responder `401` sin autenticación, `400` por body/fechas inválidos, `403` cuando el team, grupo o atleta está fuera del scope, `404` para una referencia inexistente/no ejecutada y `500` para fallas internas. Los errores SHALL usar la estructura uniforme `apierror`; no SHALL incluir información de una referencia válida cuando la otra falle.

#### Scenario: Request sin autenticación

- **WHEN** la request no contiene usuario autenticado
- **THEN** la API responde `401`

#### Scenario: Falla interna de consulta

- **WHEN** el DAO falla al obtener filas
- **THEN** la API responde el error uniforme `500` y no una respuesta A/B parcial
