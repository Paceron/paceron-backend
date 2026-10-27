# Design

## Context

`GET /ping` ya existe en `pingController` y devuelve `200 pong` sin consultar la DB. `app.StartApp` arma el router y las dependencias y luego bloquea en `router.Run(":8080")`; la configuración de entorno se carga en `config` al iniciar el proceso.

Render Free suspende un web service luego de 15 minutos sin tráfico entrante. El warmup apunta al dominio HTTPS público, no a `localhost`, para que la request vuelva a entrar por el routing de Render. Ver `proposal.md` para el propósito y `specs/server-warmup/spec.md` para el contrato observable.

## Goals / Non-Goals

**Goals:**

- Mantener un worker configurable con destino predeterminado, sin bloquear el arranque ni las requests normales.
- Hacer un GET inicial y repetirlo con un intervalo configurable, recuperándose de fallas.
- Mantener `/ping` como único endpoint de salud público.

**Non-Goals:**

- No despertar una instancia que Render ya haya suspendido: el worker no existe mientras el proceso no corre.
- No agregar dependencias, scheduler externo, cron público ni lógica de DB al endpoint de ping.

## Decisions

### D1: Worker en infraestructura y lifecycle en el bootstrap

El ciclo HTTP vive en `cmd/api/infrastructure/warmup`, separado del controller de salud. `app.StartApp` lo inicia en background después de construir el router y antes de servirlo. El proceso del servidor es su lifecycle; un contexto cancelable permite terminarlo en tests y al salir del bootstrap.

**Alternativa descartada:** iniciar el timer dentro de `PingController`. Cada request manual volvería a crear timers o mezclaría el handler con una tarea de proceso.

### D2: Configuración tipada y destino por defecto

`config` expone una configuración de warmup leída del entorno: `WARMUP_BASE_URL` (default `https://paceron-backend-as9c.onrender.com`) y `WARMUP_INTERVAL_MINUTES` (default `13`). El worker normaliza la barra final y agrega `/ping`. Un origen inválido deshabilita solo el warmup y se registra con warning; nunca bloquea el servidor.

`render.yaml` declara las variables para producción y develop. Cada servicio puede sobrescribir la URL base en su entorno; si se omite, ambos usan el default pedido.

**Alternativa descartada:** hardcodear el dominio en el worker o agregar un endpoint que active el timer al ser llamado; ambas opciones reducen configurabilidad o mezclan plano de control con salud.

### D3: HTTP acotado, no bloqueante y sin retry inmediato

El worker usa un cliente HTTP estándar con timeout por request. Inicia una request en background al arrancar y luego usa un ticker del intervalo. Solo una respuesta 2xx se cuenta como éxito; se cierran/resuelven los response bodies. Un error se registra y no corta el worker: el próximo tick hace el siguiente intento, sin retry loop ni impacto en la API.

**Alternativa descartada:** reusar `config.LoadRestClientConfig`, que configura APIs de negocio y reintentos; el warmup tiene un destino propio, operación GET idempotente y timeout acotado.

## Risks / Trade-offs

- **No evita un cold start después de que el proceso ya duerme:** Render apaga también el worker. El primer request externo sigue siendo quien despierta el servicio; el warmup ayuda mientras la instancia está activa.
- **El default de producción desde local o develop puede calentar producción:** es el default solicitado. Los despliegues pueden sobrescribir `WARMUP_BASE_URL` para apuntarse a su propio dominio.
- **Una falla HTTP no implica caída de la API:** el worker solo deja una advertencia y continúa para no convertir un mantenimiento de Render en un fallo de startup.

## Migration Plan

No hay migración de datos ni cambio de contrato HTTP. El deploy introduce el worker y declara las variables; Render puede dejar la URL sin override para usar el default o asignar la URL base propia de cada servicio. Para revertir el warmup, revertir el deploy/cambio de código.
