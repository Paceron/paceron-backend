# Spec Delta

## Purpose

Define cuándo un día del calendario de un grupo está cerrado y por lo tanto no admite
escritura, en qué zona horaria se resuelve el concepto de "hoy" para esa decisión, y bajo
qué configuración el bloqueo puede desactivarse.

## ADDED Requirements

### Requirement: El sistema SHALL resolver "hoy" contra una zona horaria de referencia del producto

La decisión de si un día del calendario está cerrado SHALL evaluarse contra la fecha y la
hora expresadas en la zona horaria de referencia del producto, y SHALL NOT contra la zona
horaria del proceso que corre el servidor ni la del cliente.

La zona de referencia SHALL ser una constante del producto, no un parámetro de despliegue:
es una decisión de negocio (en qué huso está el equipo que usa el calendario), no una
propiedad del ambiente.

El servidor SHALL comportarse igual sin importar en qué zona esté configurado el host que
lo ejecuta.

#### Scenario: Instante crítico con el servidor en UTC

Dado un servidor con zona horaria UTC, cuando la hora real del producto es el 30 de
septiembre a las 22:21, entonces "hoy" SHALL resolverse como el 30 de septiembre y una
sesión con fecha 30/09 SHALL evaluarse sobre esa fecha.

#### Scenario: El día siguiente sigue siendo futuro

Dado el mismo servidor en UTC y la misma hora real del 30/09 a las 22:21, cuando se
evalúa una sesión con fecha 1/10, entonces esa fecha SHALL evaluarse como posterior a "hoy"
y SHALL estar abierta.

#### Scenario: La zona del host no influye

Dado el mismo instante real, cuando el servidor corre con una zona horaria distinta de la
de referencia, entonces el resultado de la evaluación de un día SHALL ser idéntico al que
produce con la zona de referencia.

### Requirement: La ventana horaria de la sesión presencial SHALL evaluarse en la zona de referencia

Para una sesión presencial del día en curso, la comparación entre la hora actual y el
horario planificado de inicio SHALL realizarse con ambos valores expresados en la misma
zona horaria de referencia del producto.

El horario planificado se almacena como un instante UTC; al compararlo contra la hora
actual SHALL convertirse a la zona de referencia, no mezclar sus componentes de hora con
otra zona.

#### Scenario: Presencial que todavía no arrancó

Dado un servidor en UTC, cuando la hora del producto es el 30/09 a las 10:00 y la sesión
presencial de ese día tiene horario planificado de inicio a las 14:00, entonces el día
SHALL estar abierto.

#### Scenario: Presencial que ya arrancó

Dado un servidor en UTC, cuando la hora del producto es el 30/09 a las 18:00 y la sesión
presencial de ese día tiene horario planificado de inicio a las 14:00, entonces el día
SHALL estar cerrado.

#### Scenario: El criterio de fecha sigue manda sobre el horario

Dado un servidor en UTC, cuando la hora del producto es el 30/09 a las 18:00 y la sesión
presencial del 30/09 tiene inicio a las 14:00 pero además existe una sesión del 1/10 con
inicio a las 10:00, entonces la del 30/09 SHALL estar cerrada y la del 1/10 SHALL estar
abierta.

### Requirement: El sistema SHALL rechazar por defecto la escritura sobre un día cerrado

Con el bypass desactivado, toda operación de escritura sobre un día cerrado —crear o
actualizar el día, borrarlo, estampar un plan que lo toque, vaciar un rango o mover una
sesión— SHALL ser rechazada con el error de día cerrado, que la API expone como error de
entidad no procesable.

El bypass SHALL estar desactivado por defecto: si la variable de entorno no está definida o
no tiene un valor reconocible como verdadero, el rechazo SHALL aplicarse.

#### Scenario: Fecha pasada

Dado un sistema con el bypass desactivado, cuando se intenta escribir sobre un día con
fecha anterior a la del producto, entonces la operación SHALL ser rechazada con el error de
día cerrado.

#### Scenario: Hoy sin sesión presencial

Dado un sistema con el bypass desactivado, cuando se intenta escribir sobre el día en
curso de una sesión que no es presencial, entonces la operación SHALL ser rechazada con el
error de día cerrado.

#### Scenario: Variable ausente equivale a desactivado

Dado un sistema donde la variable de bypass no está definida, cuando se intenta escribir
sobre un día cerrado, entonces la operación SHALL ser rechazada con el error de día cerrado.

#### Scenario: Variable con valor no reconocido equivale a desactivado

Dado un sistema donde la variable de bypass tiene un valor que no es un booleano
reconocible, cuando se intenta escribir sobre un día cerrado, entonces la operación SHALL
ser rechazada con el error de día cerrado.

#### Scenario: El rechazo informa las fechas

Dado un sistema que rechaza una operación de escritura por días cerrados, cuando se
inspecciona la respuesta de error, entonces el error SHALL identificar el motivo de día
cerrado y las fechas involucradas cuando la operación las abarca (un stamp o una operación masiva las recorre enteras).

### Requirement: La configuración SHALL poder desactivar el bloqueo de días cerrados

Con el bypass activo, las operaciones de escritura sobre un día cerrado SHALL ser
aceptadas y aplicadas con normalidad, sin cambio en la forma del request ni de la response.

El bypass SHALL afectar únicamente a las decisiones de escritura. SHALL NOT alterar los
criterios con los que la API filtra o devuelve días en las lecturas.

#### Scenario: Escritura sobre día cerrado con el bypass activo

Dado un sistema con el bypass activo, cuando se intenta escribir sobre un día con fecha
anterior a la del producto, entonces la operación SHALL ser aceptada y el cambio SHALL
quedar persistido.

#### Scenario: Las lecturas no cambian

Dado un sistema con el bypass activo, cuando se consulta el calendario de un grupo, entonces
los días devueltos SHALL ser los mismos que con el bypass desactivado.

#### Scenario: El bypass es reversible sin redeploy

Dado un sistema con el bypass activo, cuando la variable pasa a estar desactivada y el
proceso se reinicia, entonces las escrituras sobre días cerrados SHALL volver a ser
rechazadas.
