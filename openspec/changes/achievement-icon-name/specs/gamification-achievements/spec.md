# Especificación: nombre explícito del ícono de logro

## Purpose

El catálogo de logros proporciona a los clientes una identidad semántica estable y un nombre explícito de glyph para representar cada logro sin inferirlo desde su ID o texto.

## ADDED Requirements

### Requirement: El catálogo expone el glyph asociado a cada logro

El sistema SHALL devolver los 21 logros del catálogo versionado con los campos `id`, `family`, `name`, `description`, `icon_key`, `icon_name`, `threshold` y `unit`. El campo `icon_name` MUST ser un string no vacío cuyo valor sea el nombre exacto de un glyph disponible en `MaterialCommunityIcons` de `@expo/vector-icons` `^15.0.3`. El campo semántico existente `icon_key` SHALL mantenerse sin cambios e igual a `id`. Los campos existentes y el shape de progreso de `GET /users/{id}/achievements` SHALL permanecer sin cambios; `icon_name` amplía de forma aditiva la respuesta de catálogo que antes solo incluía `icon_key`.

#### Scenario: Catálogo entrega un glyph válido para cada logro

- **WHEN** un usuario autenticado consulta `GET /api/v1/achievements/catalog`
- **THEN** la respuesta contiene las 21 filas del catálogo y cada fila incluye un `icon_name` no vacío que corresponde al nombre exacto de un glyph de `MaterialCommunityIcons` compatible con `@expo/vector-icons` `^15.0.3`
- **AND** cada fila conserva todos los campos existentes del catálogo

#### Scenario: La identidad semántica se conserva

- **WHEN** un consumidor recibe una fila del catálogo
- **THEN** `icon_key` continúa siendo igual a `id`
- **AND** `icon_name` identifica explícitamente la representación visual sin sustituir ni modificar el ID, la familia, el umbral o la unidad del logro
