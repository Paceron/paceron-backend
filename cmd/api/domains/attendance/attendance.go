package attendance

import "simple-arq-golang/cmd/api/domains/dbs"

// Mensajes estandarizados del registro de asistencia.
//
// La procedencia de cada fila (source, registered_by_user_id) vive en
// dbs.Attendance y no acá: este archivo es el contrato HTTP, no el modelo de
// persistencia.
const (
	// MessageRegistered indica que la asistencia se registró por primera vez (201).
	MessageRegistered = "asistencia registrada"
	// MessageAlreadyExists indica que la asistencia ya estaba registrada (200, idempotente).
	MessageAlreadyExists = "esta asistencia fue previamente registrada"
)

// QRResponse es la respuesta del endpoint de generación de QR.
//
// El QR no depende del usuario que lo pide: la URL y el PNG se derivan solo de
// (team_id, training_session_id), así que el entrenador puede reemitir el mismo QR
// y obtener exactamente los mismos bytes. Quién puede pedirlo lo valida el service
// (entrenador del equipo + sesión presencial, no cancelada y del equipo).
type QRResponse struct {
	QRCodeBase64 string `json:"qr_code_base64"` // Imagen PNG en base64
	URLEncoded   string `json:"url_encoded"`    // URL codificada en el QR
}

// SearchResponse es la respuesta del endpoint de búsqueda de asistencias.
type SearchResponse struct {
	Data []dbs.Attendance `json:"data"`
}

// RegisterResponse es la respuesta del endpoint de registro de asistencia (201/200).
//
// El 403 de "no sos miembro del grupo de esta sesión" NO se representa acá: es un
// error, y va como apierror.APIError. Este tipo solo cubre el caso exitoso.
// RegisterResponse es la respuesta del registro por QR del corredor.
//
// GroupID y SessionDate van además del mensaje porque el front, tras un
// registro exitoso, lleva al corredor directo a la sesión que acaba de registrar
// y el deep link de la pantalla de día necesita los tres: /teams/:team/groups/
// :group/calendar/:date. El QR solo trae team_id y session_instance_id, así que
// sin estos dos campos el front no puede armar la ruta y tendría que dejar al
// usuario buscando la sesión a mano.
type RegisterResponse struct {
	Message     string `json:"message"`
	GroupID     int64  `json:"group_id"`
	SessionDate string `json:"session_date"` // YYYY-MM-DD, la fecha local de la sesión
	SessionName string `json:"session_name"`
}

// SearchFilters agrupa los query params opcionales del endpoint de búsqueda.
// Se usan punteros para distinguir "ausente" de "cero"; el valor 0 (o negativo)
// lo rechaza el controller como 400 antes de llegar al service.
type SearchFilters struct {
	TeamID            *int64
	TrainingSessionID *int64
	UserID            *int64
}
