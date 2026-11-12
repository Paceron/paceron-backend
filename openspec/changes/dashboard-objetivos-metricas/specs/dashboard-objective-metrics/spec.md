# Spec Delta

## Purpose

Define los acumulados de objetivos que entrega el resumen analítico del dashboard, para que sus consumidores distingan el mejor tiempo que cumplió el objetivo y la distancia asignada frente a la lograda.

## ADDED Requirements

### Requirement: El resumen distingue el mejor tiempo que cumplió el objetivo

Al responder `GET /api/v1/dashboard/metrics/summary`, el sistema SHALL conservar `totals.best_time_ms` como el menor tiempo positivo de las series válidas. También SHALL devolver `totals.best_time_met_ms` como el menor tiempo positivo alcanzado por una serie de la que `met_target` sea `true`, evaluado con la tolerancia efectiva de esa consulta. El campo SHALL ser `null` cuando no haya una serie que cumpla el objetivo y tenga tiempo positivo.

#### Scenario: La serie más rápida no cumple un objetivo de distancia

- **WHEN** una serie válida registra 900 m en 180000 ms contra un objetivo de 1000 m y otra registra 1500 m en 220000 ms contra el mismo objetivo
- **THEN** `totals.best_time_ms` SHALL ser 180000 y `totals.best_time_met_ms` SHALL ser 220000

#### Scenario: La tolerancia efectiva determina qué serie cumplió

- **WHEN** la única serie tiene un objetivo de 4 minutos y un tiempo logrado de 250000 ms
- **THEN** con tolerancia 0.05 `totals.best_time_met_ms` SHALL ser 250000, y con tolerancia 0.01 SHALL ser `null`

#### Scenario: Ninguna serie cumplidora tiene tiempo positivo

- **WHEN** no hay una serie con `met_target` igual a `true` y tiempo positivo en el scope solicitado
- **THEN** `totals.best_time_met_ms` SHALL ser `null`

### Requirement: El resumen agrega la distancia objetivo y la distancia lograda

El sistema SHALL exponer `totals.target_distance_m` y `totals.achieved_target_distance_m` en el resumen para las series del scope cuyo tipo de ejercicio declare un objetivo de distancia y cuyo objetivo congelado de distancia sea positivo. `target_distance_m` SHALL sumar esos objetivos aunque la serie no sea válida según las reglas de medición existentes. `achieved_target_distance_m` SHALL sumar la distancia lograda positiva de las series válidas de ese mismo conjunto; una serie inválida o sin distancia lograda positiva SHALL aportar cero. Si no hay series con objetivo de distancia, ambos campos SHALL ser `null`; si hay objetivos pero ningún aporte logrado válido, el objetivo SHALL conservar su suma y el logrado SHALL ser `0`.

#### Scenario: Se incluyen objetivos sin medición válida en el denominador

- **WHEN** el scope contiene una serie válida con objetivo de 1000 m y distancia lograda de 900 m, y una serie inválida con objetivo de 500 m
- **THEN** `totals.target_distance_m` SHALL ser 1500 y `totals.achieved_target_distance_m` SHALL ser 900

#### Scenario: Hay objetivos de distancia pero ningún aporte logrado válido

- **WHEN** el scope contiene series con objetivos de distancia positivos, pero ninguna aporta una medición de distancia válida
- **THEN** `totals.target_distance_m` SHALL ser la suma de esos objetivos y `totals.achieved_target_distance_m` SHALL ser `0`

#### Scenario: El scope no contiene objetivos de distancia

- **WHEN** las series del scope no tienen un objetivo positivo de distancia declarado por su tipo de ejercicio
- **THEN** `totals.target_distance_m` y `totals.achieved_target_distance_m` SHALL ser `null`
