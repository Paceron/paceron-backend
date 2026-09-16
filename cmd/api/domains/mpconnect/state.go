package mpconnect

import (
	"fmt"
	"strconv"
	"strings"
)

// Targets posibles de retorno tras el callback de OAuth. El destino concreto
// (URL web u deep link) lo resuelve el controller contra config del servidor —
// acá solo viaja cuál de los dos es.
const (
	TargetWeb = "web"
	TargetApp = "app"
)

// El state de OAuth tiene formato "<userID>-<timestampNanos>-<target>".
//
// El target viaja adentro del state porque es el único parámetro que Mercado
// Pago devuelve tal cual se lo dimos: el redirect_uri registrado en el panel de
// MP es fijo y tiene que coincidir exactamente, así que no puede variar por
// request. Ver openspec/changes/redirect-callback-mp-connect-al-frontend/design.md.
//
// Los states de 2 segmentos ("<userID>-<timestampNanos>", el formato anterior)
// siguen siendo válidos y se interpretan como TargetWeb — así los emitidos
// antes del deploy no se rompen durante su ventana de 10 minutos.
const stateSeparator = "-"

// normalizeTarget colapsa cualquier valor desconocido a TargetWeb. Es la única
// forma de obtener un target en todo el paquete: garantiza que nunca circule un
// valor fuera del enum, que es lo que evita que esto sea un open redirect.
func normalizeTarget(target string) string {
	if target == TargetApp {
		return TargetApp
	}
	return TargetWeb
}

// BuildState arma el state CSRF para la autorización de Mercado Pago.
//
// Asume userID y timestamp positivos — lo son por construcción (el userID es
// la PK autoincremental de la sesión autenticada). Un userID negativo
// generaría un state que ParseState rechaza, porque el "-" del signo se
// confunde con el separador.
func BuildState(userID int64, timestampNanos int64, target string) string {
	return fmt.Sprintf("%d%s%d%s%s", userID, stateSeparator, timestampNanos, stateSeparator, normalizeTarget(target))
}

// ParseState descompone un state en sus partes. Acepta tanto el formato de 3
// segmentos como el legacy de 2 (que resuelve a TargetWeb).
func ParseState(state string) (userID int64, timestampNanos int64, target string, err error) {
	parts := strings.SplitN(state, stateSeparator, 3)
	if len(parts) < 2 {
		return 0, 0, "", fmt.Errorf("formato de state inválido")
	}

	userID, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("formato de state inválido")
	}

	timestampNanos, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("formato de state inválido")
	}

	target = TargetWeb
	if len(parts) == 3 {
		target = normalizeTarget(parts[2])
	}

	return userID, timestampNanos, target, nil
}

// TargetFromState devuelve a dónde hay que volver. No falla nunca: un state
// corrupto resuelve a TargetWeb, porque el redirect de error también tiene que
// llegar a algún lado — si esto devolviera error, un state ilegible dejaría al
// usuario sin ninguna pantalla de resultado.
func TargetFromState(state string) string {
	_, _, target, err := ParseState(state)
	if err != nil {
		return TargetWeb
	}
	return target
}
