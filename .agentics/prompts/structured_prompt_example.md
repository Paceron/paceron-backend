# Objetivo

Implementar un nuevo flujo llamado `example weather` que consuma la API pública de Open-Meteo desde nuestro backend respetando completamente la arquitectura actual del proyecto.

La implementación debe ser punta a punta y atravesar todas las capas existentes del proyecto utilizando:

* handlers/controllers
* services
* clients
* interfaces
* DTOs/request/response
* configuración
* logger
* restclient existente

NO debes romper la arquitectura actual ni bypassear capas.

---

# Endpoint externo a consumir

GET:

https://api.open-meteo.com/v1/forecast

Ejemplo:

https://api.open-meteo.com/v1/forecast?latitude=-31.42&longitude=-64.18&current_weather=true

---

# Parámetros

Los siguientes parámetros deben recibirse desde el request hacia nuestro servidor y propagarse al endpoint externo:

* latitude
* longitude
* current_weather

---

# Naming

Todo el flujo debe llamarse:

`example weather`

Usar esta premisa para nombrar:

* endpoints
* handlers
* services
* interfaces
* DTOs
* request/response
* archivos
* métodos
* logs

Mantener consistencia de naming en todo el flujo.

---

# Restricciones importantes

## NO agregar paquetes nuevos

No puedes agregar nuevas dependencias externas al proyecto.

Sí puedes:

* agregar structs
* interfaces
* DTOs
* requests/responses
* archivos
* packages internos
* configs
* adapters

Pero NO librerías nuevas.

---

# RestClient

Debes reutilizar obligatoriamente el `restclient` existente del proyecto.

NO crear un nuevo cliente HTTP.

La llamada al endpoint externo debe pasar por el restclient actual.

---

# Configuración del RestClient

Quiero mejorar la inicialización del restclient.

Actualmente el restclient se instancia manualmente.

Ahora debe obtener configuración desde archivos:

* application-local.properties
* application-test.properties
* application-stage.properties
* application-prod.properties

La selección del environment debe respetar el mecanismo actual del proyecto.

Desde estos archivos deben obtenerse configuraciones como por ejemplo:

* timeout
* retries
* retry delay
* base url
* o cualquier otra configuración soportada actualmente por el restclient

---

# app.go

En `app.go`, antes de inyectar el restclient en los servicios, debes:

1. Leer configuración correspondiente al environment actual
2. Construir el restclient usando esas properties
3. Inyectar el restclient configurado en las capas correspondientes

---

# Logging

Debes usar el logger existente durante todo el flujo.

Agregar logs relevantes en:

* entrada del request
* validaciones
* request saliente al servicio externo
* responses exitosas
* errores
* retries
* timeouts
* respuestas inválidas
* errores internos

NO usar fmt.Println.

Usar exclusivamente el logger existente.

---

# Manejo de errores

Implementar manejo correcto de errores y códigos HTTP.

Validar:

* parámetros faltantes
* latitude inválida
* longitude inválida
* errores del proveedor externo
* timeouts
* errores de parsing
* respuestas vacías
* errores internos

Responder con códigos HTTP correctos y mensajes consistentes con la arquitectura actual.

---

# Arquitectura

Respetar estrictamente:

* separación por capas
* inyección de dependencias
* uso de interfaces
* responsabilidades únicas
* estructura actual del repositorio

NO generar lógica acoplada.

---

# DTOs

Crear request/response models apropiados.

Mapear correctamente la respuesta externa hacia nuestra response interna.

---

# Endpoint interno esperado

Crear un endpoint interno REST tipo:

GET /example/weather

Debe recibir query params:

* latitude
* longitude
* current_weather

---

# Response esperada del proveedor externo

{
"latitude": -31.4587,
"longitude": -64.19354,
"generationtime_ms": 0.06639957427978516,
"utc_offset_seconds": 0,
"timezone": "GMT",
"timezone_abbreviation": "GMT",
"elevation": 394.0,
"current_weather_units": {
"time": "iso8601",
"interval": "seconds",
"temperature": "°C",
"windspeed": "km/h",
"winddirection": "°",
"is_day": "",
"weathercode": "wmo code"
},
"current_weather": {
"time": "2026-05-17T20:45",
"interval": 900,
"temperature": 11.9,
"windspeed": 5.7,
"winddirection": 171,
"is_day": 1,
"weathercode": 0
}
}

---

# Entregable esperado

Quiero que generes:

* todos los archivos necesarios
* código completo
* implementaciones completas
* wiring en app.go
* configuración por environment
* interfaces
* handlers
* services
* clients
* DTOs
* validaciones
* logs
* manejo de errores

Todo debe quedar listo para compilar y ejecutar.
