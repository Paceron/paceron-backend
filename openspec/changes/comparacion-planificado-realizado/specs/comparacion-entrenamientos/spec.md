## ADDED Requirements

### Requirement: Detalle planificado vs realizado de una sesión

El sistema MUST devolver, para una `session_instance` y un atleta, cada ejercicio planificado con su objetivo (series, minutos, distancia, velocidad, tipo e intensidad) y cada serie planificada con lo que registró el atleta. El sistema MUST ordenar los ejercicios como están en la sesión.

#### Scenario: Sesión completa, parcial y salteada

- **WHEN** el atleta completó la primera caminata de 10 minutos, hizo 5 minutos de la segunda y salteó la tercera
- **THEN** el cumplimiento por ejercicio es 100%, 50% y 0%
- **AND** el cumplimiento de la sesión es 50%
- **AND** la desviación de la segunda serie es de -300000 ms

#### Scenario: Serie sin registro

- **WHEN** un ejercicio planifica 2 series y el atleta registró solo la primera
- **THEN** la segunda vuelve con estado `unregistered` y cumplimiento 0%

#### Scenario: Instancia inexistente

- **WHEN** se pide el detalle de una instancia que no existe
- **THEN** el backend responde `404`

### Requirement: Calcular el cumplimiento por serie con tope de 100%

El sistema MUST calcular el cumplimiento de cada serie completada como la duración real sobre la planificada; si el ejercicio no tiene minutos, como la distancia real sobre la planificada; y si no tiene ninguno de los dos objetivos, como 100%. El sistema MUST limitar cada serie a 100% y MUST contar como 0% las series salteadas o sin registro.

#### Scenario: Objetivo de distancia

- **WHEN** el ejercicio planifica 1000 m sin minutos y el atleta registró 750 m
- **THEN** el cumplimiento de la serie es 75%

#### Scenario: Más de lo planificado

- **WHEN** el atleta corrió 12 minutos de una serie de 10
- **THEN** el cumplimiento de la serie es 100%
- **AND** la desviación es de +120000 ms

#### Scenario: Ejercicio sin objetivo

- **WHEN** un ejercicio no tiene minutos ni distancia y el atleta lo completó
- **THEN** el cumplimiento de la serie es 100%
- **AND** la desviación es `null`

### Requirement: Normalizar el feedback antes de comparar

El sistema MUST ignorar el feedback borrado, MUST pasar a base 1 las series de un ejercicio que tenga alguna serie con número 0, y MUST quedarse con una sola fila por serie, priorizando la del propio atleta.

#### Scenario: Feedback viejo en base 0

- **WHEN** un ejercicio tiene filas con `set_number` 0 y 1
- **THEN** se comparan como las series 1 y 2

#### Scenario: Serie cargada por el atleta y por el entrenador

- **WHEN** la misma serie tiene una fila del atleta y otra del entrenador
- **THEN** se usa la del atleta

#### Scenario: Series de más

- **WHEN** el atleta registró 3 series de un ejercicio que planifica 2
- **THEN** la tercera vuelve con `extra = true`
- **AND** no cambia el cumplimiento

### Requirement: Listar el cumplimiento de un rango de fechas

El sistema MUST devolver las sesiones de los días de entrenamiento del rango de los grupos donde el atleta era miembro en cada fecha, más las sesiones con feedback del atleta en el rango que no tengan un día así. Cada sesión MUST incluir su cumplimiento y sus totales, y la respuesta MUST incluir la cantidad de sesiones, la cantidad con actividad y el cumplimiento promedio. El promedio MUST tomar solo las sesiones de días anteriores a hoy y las de hoy que ya terminaron.

#### Scenario: Sesión planificada que no se hizo

- **WHEN** el atleta tenía un día de entrenamiento anterior a hoy y no registró nada
- **THEN** la sesión aparece con cumplimiento 0%, estado `none` y `counts_for_average = true`

#### Scenario: Sesión de hoy todavía no terminada

- **WHEN** la sesión es de hoy y su estado no es `finished`
- **THEN** aparece en la lista con `counts_for_average = false`
- **AND** no entra en `avg_compliance_pct` ni en `sessions_averaged`

#### Scenario: Sesión de hoy terminada

- **WHEN** la sesión es de hoy y su estado es `finished`
- **THEN** entra en el promedio como las de días anteriores

#### Scenario: Día cancelado

- **WHEN** un día del rango está cancelado
- **THEN** no aparece en la lista

#### Scenario: Grupo que el atleta dejó

- **WHEN** el atleta dejó un grupo a mitad del rango
- **THEN** aparecen los días de ese grupo anteriores a la salida y no los posteriores

#### Scenario: Filtro por tipo

- **WHEN** se pide `kind=liviano`
- **THEN** solo vuelven las sesiones con al menos un ejercicio de ese tipo

#### Scenario: Rango inválido

- **WHEN** falta una fecha, `date_from` es mayor que `date_to`, el rango supera 366 días o termina después de hoy
- **THEN** el backend responde `400` con `code = INVALID_QUERY`

### Requirement: Restringir el acceso a la comparación

El sistema MUST permitir el detalle propio solo si el atleta tiene acceso a la instancia. El sistema MUST permitir consultar a otro atleta solo al dueño del equipo de la sesión, o del equipo indicado en la lista, y solo si ese atleta pertenece al equipo.

#### Scenario: Sesión ajena

- **WHEN** un corredor pide el detalle de una sesión de un grupo al que no pertenece
- **THEN** el backend responde `403`

#### Scenario: Entrenador consulta a su corredor

- **WHEN** el dueño del equipo pide la lista con `team_id` de su equipo y `athlete_user_id` de un corredor de ese equipo
- **THEN** el backend responde `200` con las sesiones de ese corredor

#### Scenario: Otro atleta sin equipo

- **WHEN** se pide la lista con `athlete_user_id` de otro usuario y sin `team_id`
- **THEN** el backend responde `400`
