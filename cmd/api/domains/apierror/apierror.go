package apierror

type APIError struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	// Details lleva datos estructurados del error cuando el mensaje solo no
	// alcanza para que el cliente reaccione bien. Es aditivo y con omitempty a
	// propósito: los endpoints que no lo usan no cambian su respuesta.
	//
	// Hoy lo usa el 422 de la carga masiva de asistencia, que tiene que
	// identificar QUÉ user_id no eran miembros del grupo: un mensaje genérico
	// obligaría al frontend a adivinar qué filas marcar en la grilla.
	Details map[string]interface{} `json:"details,omitempty"`
}
