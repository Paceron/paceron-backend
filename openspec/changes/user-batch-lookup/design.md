## Context

Gap 2 de `paceron-frontend/docs/BACKEND_API_GAPS.md`, baja prioridad (optimización, no bloqueante) pero simple de cerrar ahora que `feature/user-search-endpoint` ya dejó el DTO acotado (`SearchResultItem`) y el criterio de autorización (login sin rol adicional) para exponer datos discretos de otros usuarios.

## Goals / Non-Goals

**Goals:**
- Resolver el roster de equipo/grupo en una sola llamada, reemplazando el fan-out N+1 del cliente.

**Non-Goals:**
- Filtrar por `status` — a diferencia de `/users/search`, ver decisión 2.
- Paginación — 50 ids alcanza para cualquier roster real del dominio (equipos de running, no miles de miembros).
- Devolver error si algún id no existe — ver decisión 3.

## Decisions

### 1. Reusa `SearchResultItem`, no duplica el DTO
**Por qué**: el shape que necesita el roster (`user_id`/`name`/`surname`/`email`) es exactamente el mismo que ya expone `/users/search`. Duplicar el tipo por tener un nombre de endpoint distinto sería una abstracción falsa — mismo dato, mismo nivel de exposición (deliberadamente sin DNI/teléfono/dirección).

### 2. Sin filtro de `status`
**Por qué**: `/users/search` filtra `status = active` porque busca candidatos nuevos para invitar — no tiene sentido sugerir a alguien bloqueado. Acá el caso de uso es distinto: el caller ya tiene los `user_id` porque salen de un roster de equipo/grupo al que pertenece (`TeamUserResponse`/`GroupUserResponse`), así que son miembros conocidos — filtrarlos por estado escondería nombres del roster real (ej. un corredor en pausa seguiría apareciendo en el roster del equipo, y su nombre tiene que resolverse igual).

### 3. Ids inexistentes se omiten, no es error
**Por qué**: es el comportamiento estándar de un `WHERE id IN (...)` — no hay forma de saber si un id "no existe" fue un error del cliente o simplemente un usuario borrado/inaccesible más adelante. Devolver 404 por lote entero sería frágil (un id viejo en un roster rompería toda la consulta); el cliente puede comparar `len(results)` contra los ids pedidos si necesita detectarlo.

### 4. Sin restricción de rol, solo login (mismo criterio que `/users/search`)
**Por qué**: igual que el search, no hay una relación previa que autorizar de forma distinta a "estar logueado" — de hecho acá es más laxo el riesgo, porque el caller ya conoce los ids (no está enumerando la base de usuarios a ciegas).

## Risks / Trade-offs

- **Enumeración de ids**: con login alcanza para pedir cualquier rango de ids, no solo los de un roster propio — mismo trade-off ya aceptado en `/users/search` (login sin rol adicional), y el DTO devuelto es igual de acotado. Si se necesita restringir a "ids que pertenecen a un roster del caller" más adelante, es un cambio de autorización posterior, no bloquea esta iteración.
