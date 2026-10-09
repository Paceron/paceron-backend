package sessionmessage

// Enums de session_messages (string sin constraint en DB; el POST valida).
const (
	TypeMessageInfo   = "info"
	TypeMessageAviso  = "aviso"
	TypeMessageAlerta = "alerta"
)

// Modos de destinatario del POST.
const (
	RecipientModeAll      = "all"
	RecipientModeMultiple = "multiple"
	RecipientModeDirect   = "direct"
)

// Roles del emisor, derivados al persistir (owner del team → trainer).
const (
	SenderRoleTrainer = "trainer"
	SenderRoleRunner  = "runner"
)

// Tope de tamaño del cuerpo del mensaje (design.md D6).
const MaxBodyLength = 2000

// SendMessageRequest es el body de POST /api/v1/session-instances/:id/messages.
// recipient_user_ids solo aplica para multiple (>= 2) y direct (exactamente 1).
type SendMessageRequest struct {
	Type             string  `json:"type" binding:"required"`
	RecipientMode    string  `json:"recipient_mode" binding:"required"`
	RecipientUserIDs []int64 `json:"recipient_user_ids"`
	Body             string  `json:"body" binding:"required"`
	ReplyToMessageID *int64  `json:"reply_to_message_id"`
}
