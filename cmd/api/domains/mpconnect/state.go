package mpconnect

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

// El state de OAuth firmado tiene formato "<payload>.<hmac-hex>", con payload
// "<userID>-<timestampNanos>-<target>". La firma (HMAC-SHA256 con el secret
// JWT del server, que llega como parámetro porque domains no importa config)
// es lo que impide forjar un state con el userID de otro: sin ella, quien
// inicia el flujo podía completar el callback con "victimID-ahora-web" y dejar
// SU cuenta de MP vinculada a la víctima.
//
// El target viaja adentro del state porque es el único parámetro que Mercado
// Pago devuelve tal cual se lo dimos: el redirect_uri registrado en el panel de
// MP es fijo y tiene que coincidir exactamente, así que no puede variar por
// request. Ver openspec/changes/redirect-callback-mp-connect-al-frontend/design.md.
const (
	stateSeparator     = "-"
	stateHmacSeparator = "."
)

// errInvalidState es el único error que devuelve ParseState: formato podrido y
// firma inválida son indistinguibles desde afuera a propósito — el callback es
// público, y el mapper del controller ya traduce este mensaje a 400.
var errInvalidState = fmt.Errorf("formato de state inválido")

// normalizeTarget colapsa cualquier valor desconocido a TargetWeb. Es la única
// forma de obtener un target en todo el paquete: garantiza que nunca circule un
// valor fuera del enum, que es lo que evita que esto sea un open redirect.
func normalizeTarget(target string) string {
	if target == TargetApp {
		return TargetApp
	}
	return TargetWeb
}

func signPayload(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// BuildState arma el state CSRF firmado para la autorización de Mercado Pago.
//
// Asume userID y timestamp positivos — lo son por construcción (el userID es
// la PK autoincremental de la sesión autenticada). Un userID negativo
// generaría un state que ParseState rechaza, porque el "-" del signo se
// confunde con el separador.
func BuildState(secret string, userID int64, timestampNanos int64, target string) string {
	payload := fmt.Sprintf("%d%s%d%s%s", userID, stateSeparator, timestampNanos, stateSeparator, normalizeTarget(target))
	return payload + stateHmacSeparator + signPayload(secret, payload)
}

// ParseState descompone un state en sus partes. Verifica la firma ANTES de
// parsear el payload: se rechaza sin distinción un state sin firma (como el
// formato anterior de 2 segmentos, dejado de emitir con este cambio — los que
// quedaron en vuelo mueren en su ventana de 10 min, igual que con una rotación
// de secret), con firma que no coincide, o con payload malformado.
func ParseState(secret string, state string) (userID int64, timestampNanos int64, target string, err error) {
	idx := strings.LastIndex(state, stateHmacSeparator)
	if idx < 0 {
		return 0, 0, "", errInvalidState
	}
	payload, macHex := state[:idx], state[idx+len(stateHmacSeparator):]

	if !hmac.Equal([]byte(signPayload(secret, payload)), []byte(macHex)) {
		return 0, 0, "", errInvalidState
	}

	parts := strings.SplitN(payload, stateSeparator, 3)
	if len(parts) != 3 {
		return 0, 0, "", errInvalidState
	}

	userID, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, "", errInvalidState
	}

	timestampNanos, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", errInvalidState
	}

	return userID, timestampNanos, normalizeTarget(parts[2]), nil
}

// TargetFromState devuelve a dónde hay que volver. No falla nunca ni verifica
// la firma: el redirect de error tiene que funcionar aunque el state sea
// justamente el problema, y el target se colapsa al enum antes de usarse — no
// existe destino fuera de las URLs configuradas en el server.
func TargetFromState(state string) string {
	payload := state
	if idx := strings.LastIndex(state, stateHmacSeparator); idx >= 0 {
		payload = state[:idx]
	}
	parts := strings.SplitN(payload, stateSeparator, 3)
	if len(parts) != 3 {
		return TargetWeb
	}
	return normalizeTarget(parts[2])
}
