package dbs

import "time"

// SessionMessage es un mensaje enviado por el entrenador en el chat de una sesión
// instanciada. SessionInstanceID es FK opaca al patrón del repo (sin constraint
// física, como attendance.training_session_id) pero con índice para el GET por
// sesión. Los mensajes son inmutables: no hay updated_at ni deleted_at (no se
// editan ni se borran). SenderRole: 'trainer' | 'runner' — Type: 'info' |
// 'aviso' | 'alerta' — RecipientMode: 'all' | 'multiple' | 'direct'.
type SessionMessage struct {
	ID                int64     `gorm:"column:id;primaryKey"`                      // ID del mensaje (autoincremental)
	SessionInstanceID int64     `gorm:"column:session_instance_id;not null;index"` // Sesión instanciada (FK opaca, sin constraint)
	SenderUserID      int64     `gorm:"column:sender_user_id;not null"`            // Usuario que envía
	SenderRole        string    `gorm:"column:sender_role;not null"`               // Rol del emisor: 'trainer' | 'runner'
	Type              string    `gorm:"column:type;not null"`                      // Tipo: 'info' | 'aviso' | 'alerta'
	RecipientMode     string    `gorm:"column:recipient_mode;not null"`            // Modo de envío: 'all' | 'multiple' | 'direct'
	Body              string    `gorm:"column:body;not null"`                      // Contenido del mensaje
	ReplyToMessageID  *int64    `gorm:"column:reply_to_message_id"`                // Mensaje citado (nullable, sin FK)
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime"`          // Fecha de envío
}

func (SessionMessage) TableName() string {
	return "session_messages"
}
