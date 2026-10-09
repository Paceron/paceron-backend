package sessionmessage

import "time"

// SessionMessageResponse es el shape plano confirmado con el frontend (D8):
// sin omitempty, recipient_user_ids devuelve [] cuando el modo es all.
type SessionMessageResponse struct {
	ID                int64     `json:"id"`
	SessionInstanceID int64     `json:"session_instance_id"`
	SenderUserID      int64     `json:"sender_user_id"`
	SenderRole        string    `json:"sender_role"`
	Type              string    `json:"type"`
	RecipientMode     string    `json:"recipient_mode"`
	RecipientUserIDs  []int64   `json:"recipient_user_ids"`
	Body              string    `json:"body"`
	ReplyToMessageID  *int64    `json:"reply_to_message_id"`
	CreatedAt         time.Time `json:"created_at"`
}

// MessagesListResponse es la respuesta del GET por sesión (D7).
type MessagesListResponse struct {
	Messages []SessionMessageResponse `json:"messages"`
}
