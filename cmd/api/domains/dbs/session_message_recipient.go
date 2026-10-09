package dbs

// SessionMessageRecipient registra en qué modo recibió un mensaje un usuario
// (una fila por destinatario en modo 'multiple', una para 'direct', ninguna fila
// o la totalidad por convención en 'all'). La PK es compuesta (message_id,
// user_id) y ambas columnas quedan sin FK física; el índice por user_id soporta
// lookups futuros por destinatario.
type SessionMessageRecipient struct {
	MessageID int64 `gorm:"primaryKey;column:message_id;priority:1"`
	UserID    int64 `gorm:"primaryKey;column:user_id;priority:2;index"` // PK compuesta; índice para lookups por destinatario
}

func (SessionMessageRecipient) TableName() string {
	return "session_message_recipients"
}
