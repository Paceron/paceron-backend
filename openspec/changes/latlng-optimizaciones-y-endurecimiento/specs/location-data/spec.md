# Spec delta: location-data (Gap 29 — ubicación con lat/lng y charset de dirección)

> Nota de convención del repo (AGENTS.md §3): las capabilities históricas no
> estaban archivadas, así que los requirements se declaran completos bajo
> `## ADDED Requirements` — ningún otro doc define hoy este comportamiento.

## ADDED Requirements

### Requirement: Ubicación del usuario con lat/lng

El sistema DEBE cumplir lo siguiente (MUST):

- El usuario registra y actualiza su ubicación con `lat` y `lng` opcionales
  (nullable), además de los campos `country`/`province`/`city`/`street`/
  `number` que ya existían; ambos endpoints (registro y update) aceptan
  `lat`/`lng` y las respuestas de usuario los devuelven siempre (null si no
  se setearon, sin omitempty... con el criterio del paquete que use hoy para
  campos nuevos de dirección).
- El service valida el rango: `lat` entre -90 y 90, `lng` entre -180 y 180;
  fuera de rango → `400` con mensaje de campo inválido.
- Sin backfill: los registros anteriores a este cambio quedan con
  `lat`/`lng` en `null` y el contrato las devuelve igual.

#### Scenario: registro con ubicación del picker

- **WHEN** un usuario se registra mandando `lat`/`lng` dentro de rango junto
  a los campos de dirección
- **THEN** la cuenta guarda el par y `GET /auth/user` (y el update response)
  lo devuelve tal cual con los campos de dirección intactos.

#### Scenario: lat/lng fuera de rango se rechaza

- **WHEN** el body trae `lat = 95.4` o `lng = -200`
- **THEN** el endpoint responde `400` con el mensaje de rango de ese campo,
  sin escribir nada en la base.

#### Scenario: lat/lng omitidos quedan en null

- **WHEN** el body no incluye `lat`/`lng` (frontend sin picker o cliente
  viejo)
- **THEN** se guardan como `null` y las respuestas los muestran `null`.

### Requirement: Ubicación del equipo con lat/lng

El sistema DEBE cumplir lo siguiente (MUST):

- `POST /teams`, `PUT /teams/{id}/address` y las respuestas de equipo que ya
  exponen la dirección (`GET /teams/{id}`, GetAll, Create, Update,
  UpdateAddress) aceptan/devuelven `lat`/`lng` opcionales con el mismo
  rango y el mismo manejo `null` que la ubicación del usuario.
- `TeamSearchResult` no cambia (el buscador no expone dirección hoy).

#### Scenario: equipo se crea con punto del mapa

- **WHEN** el entrenador crea el equipo con `lat`/`lng` del picker
- **THEN** `GET /teams/{id}` los devuelve tal cual.

#### Scenario: dirección de equipo sin punto no se rompe

- **WHEN** se actualiza la dirección de un equipo viejo sin `lat`/`lng`
- **THEN** se actualizan los campos de texto y los puntos quedan como
  estaban (null si nunca se setearon).

### Requirement: Charset de campos de dirección (veraz)

El sistema DEBE cumplir lo siguiente (MUST):

- Los validadores de `country`/`province`/`city`/`street`/`number` del
  registro y del update de usuario aceptan, además del conjunto actual
  (letras con acentos, números, espacios): guion `-`, punto `.`, apóstrofe
  `'` y coma `,` (aditivo; nada que antes pasaba ahora se rechaza).
- Los mensajes de error de esos campos listan los caracteres admitidos
  reales (ya no dicen "solo letras y espacios" cuando aceptan más).

#### Scenario: nombre con apóstrofe pasa el registro

- **WHEN** el registro llega con `city = "Villa O'Higgins"` o
  `street = "San Martín – Esq. 3"` (guion/punto/apóstrofe/coma)
- **THEN** se acepta y se guarda.

#### Scenario: charset peligroso sigue rechazado

- **WHEN** un campo de dirección trae `<` `>` `;` `(` `)` o comillas dobles
- **THEN** sigue `400` con el mensaje actualizado (el relax no abre SQL ni
  HTML: sigue regex acotado).
