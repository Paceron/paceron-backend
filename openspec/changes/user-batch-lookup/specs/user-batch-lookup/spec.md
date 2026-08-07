## ADDED Requirements

### Requirement: Lookup en lote de usuarios por id
El sistema SHALL exponer `GET /api/v1/users?ids=<id1,id2,...>` para cualquier usuario autenticado, sin restricción de rol adicional, resolviendo `user_id`, `name`, `surname`, `email` de hasta 50 usuarios en una sola consulta, sin filtrar por `status`.

#### Scenario: Lookup válido con ids existentes
- **WHEN** un usuario autenticado pide `ids` con hasta 50 valores numéricos separados por coma
- **THEN** el sistema retorna HTTP 200 con un resultado por cada id encontrado, cada uno con `user_id`, `name`, `surname`, `email`, sin importar su `status`

#### Scenario: Algunos ids no existen
- **WHEN** parte de los ids pedidos no corresponden a ningún usuario
- **THEN** el sistema retorna HTTP 200 con los resultados de los ids que sí existen, sin error por los que faltan

#### Scenario: Parámetro ids ausente o vacío
- **WHEN** la solicitud no incluye `ids` o el valor es una cadena vacía
- **THEN** el sistema retorna HTTP 400 sin ejecutar la consulta

#### Scenario: Id no numérico
- **WHEN** alguno de los valores separados por coma no es un entero válido
- **THEN** el sistema retorna HTTP 400 sin ejecutar la consulta

#### Scenario: Demasiados ids
- **WHEN** se piden más de 50 ids en una sola llamada
- **THEN** el sistema retorna HTTP 400 sin ejecutar la consulta

#### Scenario: Usuario no autenticado
- **WHEN** se llama al endpoint sin `Authorization: Bearer <access_token>` válido
- **THEN** el sistema retorna HTTP 401, igual que cualquier otra ruta protegida
