# Spec Delta

## Purpose

Mantiene tráfico de salud configurable hacia el endpoint `/ping` mientras la API está activa, reduciendo el cold start por inactividad en Render.

## ADDED Requirements

### Requirement: El warmup SHALL resolver su destino y frecuencia desde configuración

El worker SHALL leer `WARMUP_BASE_URL` como origen HTTP(S) y SHALL llamar a su ruta `/ping`. Si la variable está ausente o vacía, SHALL usar `https://paceron-backend-as9c.onrender.com`. `WARMUP_INTERVAL_MINUTES` SHALL aceptar enteros positivos y usar 13 cuando falte.

#### Scenario: URL por defecto
- **WHEN** `WARMUP_BASE_URL` está ausente o vacía
- **THEN** el worker llama a `https://paceron-backend-as9c.onrender.com/ping`

#### Scenario: URL alternativa
- **WHEN** `WARMUP_BASE_URL` contiene otro origen HTTP(S) válido
- **THEN** el worker llama a la ruta `/ping` de ese origen

#### Scenario: Intervalo omitido o inválido
- **WHEN** `WARMUP_INTERVAL_MINUTES` falta, no es entero o no es positivo
- **THEN** el worker usa 13 minutos y advierte si el valor presente es inválido

#### Scenario: URL inválida
- **WHEN** `WARMUP_BASE_URL` contiene una URL que no es un origen HTTP(S) absoluto
- **THEN** el warmup queda deshabilitado con una advertencia y la API continúa arrancando

### Requirement: El worker SHALL llamar a `/ping` inmediatamente y en cada intervalo

Con un destino válido, el worker SHALL iniciar un `GET` asíncrono al arrancar la API y repetirlo según `WARMUP_INTERVAL_MINUTES`, sin bloquear el servidor HTTP.

#### Scenario: Inicio del servicio
- **WHEN** la API arranca con warmup habilitado
- **THEN** inicia una request al endpoint configurado y programa las siguientes según el intervalo

#### Scenario: Intervalo configurado
- **WHEN** `WARMUP_INTERVAL_MINUTES` vale `13`
- **THEN** las llamadas periódicas se realizan cada 13 minutos

### Requirement: Los fallos de warmup SHALL ser aislados del servicio

Una falla de red o respuesta HTTP no exitosa SHALL registrarse con nivel de error, SHALL NOT detener ni degradar el servidor y SHALL permitir que el siguiente ciclo vuelva a intentar la request.

#### Scenario: El endpoint no responde
- **WHEN** una llamada falla por timeout, error de red o estado HTTP no exitoso
- **THEN** se registra la falla y el worker continúa programado para el próximo intervalo

### Requirement: El warmup SHALL conservar el contrato público de salud

El warmup SHALL consumir el `GET /ping` existente y NO SHALL agregar una ruta pública ni modificar la respuesta actual `200 pong`.

#### Scenario: Salud pública sin cambios
- **WHEN** un cliente consulta `GET /ping`
- **THEN** recibe la respuesta existente `200 pong`, independientemente de que el warmup esté habilitado
