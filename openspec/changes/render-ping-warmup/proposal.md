# Proposal

## Why

El servicio backend en Render Free puede suspenderse tras un período sin tráfico entrante y la primera request posterior sufre un cold start. Un warmup configurable puede mantener tráfico periódico mientras la instancia está activa y reducir esa demora.

### Objetivo

Permitir que el backend consulte periódicamente su endpoint de salud `/ping` usando una URL configurable, sin agregar una dependencia externa ni lógica al cliente.

### Alcance

- Job de fondo iniciado junto con la API.
- URL base y frecuencia configurables por variables de entorno; la URL base predeterminada será `https://paceron-backend-as9c.onrender.com` y la frecuencia, 13 minutos.
- Llamada inicial al iniciar el proceso y llamadas posteriores según el intervalo.
- Apagado seguro del job cuando termina el proceso y logging acotado de éxitos y fallas.

### No alcance

- No se agrega un endpoint público nuevo: se consume `GET /ping`.
- No se garantiza despertar una instancia que Render ya suspendió; el job solo existe mientras el proceso está activo.
- No se modifica el plan de Render ni se incorpora un proveedor externo de uptime monitoring.

### Métrica de éxito

El proceso realiza una request HTTP inicial a la URL base configurada (o al destino Render predeterminado) y continúa haciéndolas al intervalo configurado, aun si una falla. Los tests verifican frecuencia, cancelación, timeout y manejo de estados HTTP sin depender de Render.

## What Changes

- Se agrega un worker configurable de warmup que hace `GET` a una URL de salud configurable.
- Se leen `WARMUP_BASE_URL` y `WARMUP_INTERVAL_MINUTES`; al origen configurado se le agrega `/ping`, la URL base predeterminada es `https://paceron-backend-as9c.onrender.com` y el intervalo usa 13 si no está definido.
- Se inicia y detiene el worker desde el bootstrap de la aplicación.
- Se declaran las variables para los servicios Render de producción y develop, dejando la URL como configuración específica del entorno.

## Capabilities

### New Capabilities

- `server-warmup`: comportamiento del proceso para generar tráfico de salud periódico a una URL base configurable, con destino Render por defecto.

### Modified Capabilities

- Ninguna.

## Impact

- Backend Go: `cmd/api/config/`, el bootstrap en `cmd/api/app/` y un componente pequeño de infraestructura para el worker HTTP.
- Render: variables de entorno de ambos servicios en `render.yaml`; la URL concreta se configura por entorno.
- Tests unitarios del scheduler/cliente y de la lectura/validación de configuración. No se agregan dependencias.
