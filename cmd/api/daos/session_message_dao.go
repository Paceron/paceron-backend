package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/sessionmessage"
)

type SessionMessageDaoInterface interface {
	Create(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error
	FindByID(ctx *gin.Context, id int64) (*dbs.SessionMessage, []int64, error)
	FindVisibleSince(ctx *gin.Context, sessionInstanceID, viewerUserID, sinceID int64) ([]dbs.SessionMessage, [][]int64, error)
}

type sessionMessageDao struct {
	DB *gorm.DB
}

func NewSessionMessageDao(database *gorm.DB) SessionMessageDaoInterface {
	return &sessionMessageDao{
		DB: database,
	}
}

// Create persiste el mensaje y sus destinatarios en la misma transacción: las
// dos escrituras juntas o nada. En mode=all llega vacío/nil y no genera filas.
// Al salir, message.ID queda poblado.
func (d *sessionMessageDao) Create(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
	err := d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(message).Error; err != nil {
			return fmt.Errorf("error creating session message: %w", err)
		}
		if len(recipientUserIDs) == 0 {
			return nil
		}
		recipients := make([]dbs.SessionMessageRecipient, 0, len(recipientUserIDs))
		for _, userID := range recipientUserIDs {
			recipients = append(recipients, dbs.SessionMessageRecipient{MessageID: message.ID, UserID: userID})
		}
		if err := tx.Create(&recipients).Error; err != nil {
			return fmt.Errorf("error creating session message recipients: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error in session message transaction: %w", err)
	}
	return nil
}

func (d *sessionMessageDao) FindByID(ctx *gin.Context, id int64) (*dbs.SessionMessage, []int64, error) {
	var message dbs.SessionMessage
	err := d.DB.First(&message, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("error finding session message by id: %w", err)
	}

	userIDs, err := d.recipientIDs(d.DB, []int64{message.ID})
	if err != nil {
		return nil, nil, err
	}
	return &message, userIDs[message.ID], nil
}

// FindVisibleSince devuelve los mensajes de la instancia con id > sinceID que el
// viewer ve (emisor, mode=all o en la lista de recipients), order por id ASC.
// Los recipient_ids se resuelven en una segunda query por el lote de la página
// (sin N+1 por mensaje); el slice devuelto es paralelo al de mensajes.
func (d *sessionMessageDao) FindVisibleSince(ctx *gin.Context, sessionInstanceID, viewerUserID, sinceID int64) ([]dbs.SessionMessage, [][]int64, error) {
	var messages []dbs.SessionMessage
	err := d.DB.
		Where("session_instance_id = ? AND id > ?", sessionInstanceID, sinceID).
		Where("sender_user_id = ? OR recipient_mode = ? OR id IN (?)", viewerUserID,
			sessionmessage.RecipientModeAll,
			d.DB.Model(&dbs.SessionMessageRecipient{}).Select("message_id").Where("user_id = ?", viewerUserID)).
		Order("id ASC").
		Find(&messages).Error
	if err != nil {
		return nil, nil, fmt.Errorf("error finding visible session messages: %w", err)
	}

	if len(messages) == 0 {
		return messages, [][]int64{}, nil
	}

	ids := make([]int64, 0, len(messages))
	for _, m := range messages {
		ids = append(ids, m.ID)
	}
	byMessage, err := d.recipientIDs(d.DB, ids)
	if err != nil {
		return nil, nil, err
	}

	recipientIDs := make([][]int64, 0, len(messages))
	for _, m := range messages {
		recipientIDs = append(recipientIDs, byMessage[m.ID])
	}
	return messages, recipientIDs, nil
}

// recipientIDs agrupa los user_ids de los mensajes dados en una sola query.
// El mapa devuelto tiene entrada (posición vacía) para cada message_id pedido.
func (d *sessionMessageDao) recipientIDs(db *gorm.DB, messageIDs []int64) (map[int64][]int64, error) {
	var rows []dbs.SessionMessageRecipient
	err := db.Where("message_id IN (?)", messageIDs).Order("user_id ASC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error finding session message recipients: %w", err)
	}

	byMessage := make(map[int64][]int64, len(messageIDs))
	for _, id := range messageIDs {
		byMessage[id] = []int64{}
	}
	for _, r := range rows {
		byMessage[r.MessageID] = append(byMessage[r.MessageID], r.UserID)
	}
	return byMessage, nil
}
