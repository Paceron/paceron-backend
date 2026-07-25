## Context

La infraestructura de mailer (`infrastructure/mailer/`) fue construida en `agregar-envio-mails-smtp` y conectada únicamente al flujo de registro. `agregar-status-usuario-y-modificaciones` dejó explícitamente fuera de alcance "Notificaciones de cambio de estado". Este cambio cierra ese gap para la transición específica a `inactive`, reutilizando toda la infraestructura existente sin tocar el cliente SMTP ni la configuración.

## Goals / Non-Goals

**Goals:**
- Enviar un correo de despedida cuando `ChangeStatus` transiciona un usuario a `inactive`
- Reusar el mismo patrón de `authService.Register`: envío best-effort, nunca bloquea la respuesta
- Evitar reenvíos redundantes cuando se llama al endpoint repetidamente con `status=inactive` sobre un usuario ya inactivo

**Non-Goals:**
- Notificaciones para otras transiciones de estado (`pause`, `blocked`, `suspended`, reactivación a `active`)
- Corregir la falta de autenticación/autorización de `PATCH /api/v1/users/:id/status` (issue conocido, separado)
- Nuevo cliente SMTP o configuración (se reutiliza `config.MySMTP` y `mailer.Client` tal cual existen)

## Decisions

### 1. Guarda contra reenvíos redundantes: comparar estado previo antes de actualizar
- **Por qué**: sin guarda, invocar el endpoint repetidamente con `status=inactive` sobre un usuario ya inactivo reenviaría el correo cada vez. Se captura `previousStatus` del `FindByID` (antes de `UpdateStatus`) y solo se envía si `previousStatus != "inactive" && status == "inactive"`.
- **Alternativa**: no guardar y aceptar reenvíos — descartada, es spam evitable con una comparación trivial ya disponible en memoria.

### 2. Ubicación del hook: dentro de `userService.ChangeStatus`, no en el controller
- **Por qué**: el controller (`user_controller.go`) no tiene lógica de negocio, solo mapea errores a códigos HTTP; toda la lógica de transición de estado ya vive en el service. Mismo criterio usado para el welcome email en `authService.Register`.

### 3. Reordenar `app.go`: mover el bloque Mailer antes de User flow
- **Por qué**: `NewUserService` ahora necesita el `mailerClient` como segundo argumento; el bloque Mailer se construía después del bloque User flow en el código anterior. Se elige mover Mailer hacia arriba (en vez de mover User flow hacia abajo) porque minimiza el diff y no afecta el orden relativo de Auth flow, que ya depende de ambos.

### 4. Mismo cliente/logger de mailer compartido entre Auth y User flow
- **Por qué**: no hay razón para instanciar dos `mailer.Client` o dos loggers; se pasa la misma instancia `mailerClient` a `NewAuthService` y `NewUserService`.

### 5. Envío best-effort, nunca bloquea la respuesta (mismo criterio que Decisión 6 de `agregar-envio-mails-smtp`)
- **Por qué**: el correo de despedida es una notificación, no un requisito para que la baja se efectivice. Un fallo de SMTP no debe impedir que el usuario quede desactivado.

### 6. Mailer nil-safe en el service, con tests dedicados esta vez (a diferencia de `agregar-envio-mails-smtp`)
- **Por qué**: `userService.mailer` puede ser `nil`, chequeado antes de usarse, para no romper los 15 call-sites existentes de `NewUserService`. A diferencia de los dos cambios anteriores (que difirieron los tests del flujo conectado), acá sí se agregó un `mockMailer` y cobertura dedicada de `ChangeStatus`, porque la guarda anti-reenvío es lógica nueva real que amerita protección contra regresiones.

## Risks / Trade-offs

- **Endpoint sin autenticación** (conocido, fuera de alcance): el correo se disparará para cualquier caller de `PATCH /api/v1/users/:id/status`, no solo para el propio usuario. No se agrava el riesgo existente, pero tampoco se mitiga.
- **Reordenamiento de `app.go`**: bajo riesgo, cambio mecánico de orden de construcción sin cambiar comportamiento de los demás flujos.
