package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/sessionmessage"
)

// Errores de negocio del módulo de mensajería de sesión. El controller mapea
// ErrSessionMessageInvalid → 400, ErrSessionMessageForbidden → 403 y
// ErrCalendarInstanceNotFound → 404 (mensaje alineado con el detalle de sesión).
var (
	ErrSessionMessageInvalid   = errors.New("datos inválidos")
	ErrSessionMessageForbidden = errors.New("no autorizado")
)

// maxBodyLen es el tope de caracteres del cuerpo del mensaje (D6).
const maxBodyLen = sessionmessage.MaxBodyLength

// SessionMessageServiceInterface define las operaciones de mensajería del chat
// de una sesión instanciada. Toda autorización aplica la regla dual de
// HasInstanceAccess (session-instance-detail D2): 404 si la instancia no
// existe, 403 si el caller no tiene acceso.
type SessionMessageServiceInterface interface {
	Create(ctx *gin.Context, authUserID, sessionInstanceID int64, req sessionmessage.SendMessageRequest) (*sessionmessage.SessionMessageResponse, error)
	List(ctx *gin.Context, authUserID, sessionInstanceID, sinceID int64) (*sessionmessage.MessagesListResponse, error)
}

type sessionMessageService struct {
	sessionMessageDao  daos.SessionMessageDaoInterface
	sessionInstanceDao daos.SessionInstanceDaoInterface
	calendarDayDao     daos.GroupCalendarDaoInterface
	groupDao           daos.GroupDaoInterface
	teamDao            daos.TeamDaoInterface
}

// NewSessionMessageService crea una nueva instancia de SessionMessageService.
func NewSessionMessageService(sessionMessageDao daos.SessionMessageDaoInterface, sessionInstanceDao daos.SessionInstanceDaoInterface, calendarDayDao daos.GroupCalendarDaoInterface, groupDao daos.GroupDaoInterface, teamDao daos.TeamDaoInterface) SessionMessageServiceInterface {
	return &sessionMessageService{
		sessionMessageDao:  sessionMessageDao,
		sessionInstanceDao: sessionInstanceDao,
		calendarDayDao:     calendarDayDao,
		groupDao:           groupDao,
		teamDao:            teamDao,
	}
}

// Create valida el request, deriva sender_role del owner del team del día de
// la instancia y persiste mensaje + recipients transaccionalmente vía el DAO.
func (s *sessionMessageService) Create(ctx *gin.Context, authUserID, sessionInstanceID int64, req sessionmessage.SendMessageRequest) (*sessionmessage.SessionMessageResponse, error) {
	sessionInstance, err := s.sessionInstanceDao.FindByID(ctx, sessionInstanceID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar sesión instancia: %w", err)
	}
	if sessionInstance == nil {
		return nil, ErrCalendarInstanceNotFound
	}
	if err := s.checkInstanceAccess(ctx, authUserID, sessionInstanceID); err != nil {
		return nil, err
	}

	body, recipientUserIDs, err := validateSendMessage(req)
	if err != nil {
		return nil, err
	}
	if len(recipientUserIDs) > 0 {
		if err := s.validateRecipients(ctx, sessionInstanceID, recipientUserIDs); err != nil {
			return nil, err
		}
	}

	replyTo, err := s.validateReply(ctx, authUserID, sessionInstanceID, req.ReplyToMessageID)
	if err != nil {
		return nil, err
	}

	senderRole, err := s.deriveSenderRole(ctx, authUserID, sessionInstanceID)
	if err != nil {
		return nil, err
	}

	message := &dbs.SessionMessage{
		SessionInstanceID: sessionInstanceID,
		SenderUserID:      authUserID,
		SenderRole:        senderRole,
		Type:              req.Type,
		RecipientMode:     req.RecipientMode,
		Body:              body,
		ReplyToMessageID:  replyTo,
	}
	if err := s.sessionMessageDao.Create(ctx, message, recipientUserIDs); err != nil {
		return nil, fmt.Errorf("error al crear mensaje de sesión: %w", err)
	}

	if recipientUserIDs == nil {
		recipientUserIDs = []int64{}
	}
	resp := toSessionMessageResponse(message, recipientUserIDs)
	return &resp, nil
}

// List devuelve los mensajes de la instancia con id > since que el caller ve
// (D7): la visibilidad la filtra el DAO, la respuesta no se re-filtra.
func (s *sessionMessageService) List(ctx *gin.Context, authUserID, sessionInstanceID, sinceID int64) (*sessionmessage.MessagesListResponse, error) {
	sessionInstance, err := s.sessionInstanceDao.FindByID(ctx, sessionInstanceID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar sesión instancia: %w", err)
	}
	if sessionInstance == nil {
		return nil, ErrCalendarInstanceNotFound
	}
	if err := s.checkInstanceAccess(ctx, authUserID, sessionInstanceID); err != nil {
		return nil, err
	}

	messages, recipientIDs, err := s.sessionMessageDao.FindVisibleSince(ctx, sessionInstanceID, authUserID, sinceID)
	if err != nil {
		return nil, fmt.Errorf("error al listar mensajes de sesión: %w", err)
	}

	response := make([]sessionmessage.SessionMessageResponse, 0, len(messages))
	for i := range messages {
		response = append(response, toSessionMessageResponse(&messages[i], recipientIDs[i]))
	}
	return &sessionmessage.MessagesListResponse{Messages: response}, nil
}

// checkInstanceAccess aplica el guard dual: 403 si el caller no tiene acceso a
// la instancia (el 404 ya se resolvió antes con FindByID).
func (s *sessionMessageService) checkInstanceAccess(ctx *gin.Context, authUserID, sessionInstanceID int64) error {
	access, err := s.sessionInstanceDao.HasInstanceAccess(ctx, sessionInstanceID, authUserID)
	if err != nil {
		return fmt.Errorf("error al chequear acceso a sesión instancia: %w", err)
	}
	if !access {
		return ErrSessionMessageForbidden
	}
	return nil
}

// validateSendMessage valida type/mode/body/destinatarios según D6. Devuelve el
// body trimmeado y la lista de destinatarios normalizada (nil para all).
func validateSendMessage(req sessionmessage.SendMessageRequest) (string, []int64, error) {
	switch req.Type {
	case sessionmessage.TypeMessageInfo, sessionmessage.TypeMessageAviso, sessionmessage.TypeMessageAlerta:
	default:
		return "", nil, fmt.Errorf("%w: type debe ser info, aviso o alerta", ErrSessionMessageInvalid)
	}

	switch req.RecipientMode {
	case sessionmessage.RecipientModeAll:
		if len(req.RecipientUserIDs) > 0 {
			return "", nil, fmt.Errorf("%w: recipient_user_ids debe estar vacío cuando recipient_mode es all", ErrSessionMessageInvalid)
		}
	case sessionmessage.RecipientModeDirect:
		if len(req.RecipientUserIDs) != 1 {
			return "", nil, fmt.Errorf("%w: direct requiere exactamente 1 destinatario", ErrSessionMessageInvalid)
		}
	case sessionmessage.RecipientModeMultiple:
		if len(req.RecipientUserIDs) < 2 {
			return "", nil, fmt.Errorf("%w: multiple requiere al menos 2 destinatarios", ErrSessionMessageInvalid)
		}
	default:
		return "", nil, fmt.Errorf("%w: recipient_mode debe ser all, multiple o direct", ErrSessionMessageInvalid)
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		return "", nil, fmt.Errorf("%w: body es obligatorio", ErrSessionMessageInvalid)
	}
	if len(body) > maxBodyLen {
		return "", nil, fmt.Errorf("%w: body no puede superar los %d caracteres", ErrSessionMessageInvalid, maxBodyLen)
	}
	return body, req.RecipientUserIDs, nil
}

// validateRecipients chequea que cada destinatario sea participante de la
// sesión (misma regla dual que el emisor; el fallo es 400, D6).
func (s *sessionMessageService) validateRecipients(ctx *gin.Context, sessionInstanceID int64, recipientUserIDs []int64) error {
	for _, userID := range recipientUserIDs {
		access, err := s.sessionInstanceDao.HasInstanceAccess(ctx, sessionInstanceID, userID)
		if err != nil {
			return fmt.Errorf("error al chequear acceso a sesión instancia: %w", err)
		}
		if !access {
			return fmt.Errorf("%w: el destinatario %d no participa de la sesión", ErrSessionMessageInvalid, userID)
		}
	}
	return nil
}

// validateReply asegura que el mensaje citado exista, sea de la misma sesión y
// sea visible para el emisor (emisor, mode=all o recipient). Cualquier fallo es
// 400 con el motivo indicado.
func (s *sessionMessageService) validateReply(ctx *gin.Context, authUserID, sessionInstanceID int64, replyToMessageID *int64) (*int64, error) {
	if replyToMessageID == nil {
		return nil, nil
	}
	if *replyToMessageID <= 0 {
		return nil, fmt.Errorf("%w: reply_to_message_id debe ser un número entero mayor a 0", ErrSessionMessageInvalid)
	}
	replyMessage, recipients, err := s.sessionMessageDao.FindByID(ctx, *replyToMessageID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar mensaje citado: %w", err)
	}
	if replyMessage == nil {
		return nil, fmt.Errorf("%w: el mensaje citado no existe", ErrSessionMessageInvalid)
	}
	if replyMessage.SessionInstanceID != sessionInstanceID {
		return nil, fmt.Errorf("%w: el mensaje citado no pertenece a esta sesión", ErrSessionMessageInvalid)
	}
	visible := replyMessage.SenderUserID == authUserID || replyMessage.RecipientMode == sessionmessage.RecipientModeAll
	if !visible && recipients != nil {
		for _, userID := range recipients {
			if userID == authUserID {
				visible = true
				break
			}
		}
	}
	if !visible {
		return nil, fmt.Errorf("%w: el mensaje citado no es visible para el emisor", ErrSessionMessageInvalid)
	}
	return replyToMessageID, nil
}

// deriveSenderRole resuelve 'trainer' solo si el emisor es el owner del team
// del grupo del día de la instancia; 'runner' en cualquier otro caso. Los
// caminos sin día/handicap de cadena caen en runner: el rol es descriptivo.
func (s *sessionMessageService) deriveSenderRole(ctx *gin.Context, senderUserID, sessionInstanceID int64) (string, error) {
	day, err := s.calendarDayDao.FindBySessionInstanceID(ctx, sessionInstanceID)
	if err != nil {
		return "", fmt.Errorf("error al buscar día de la instancia: %w", err)
	}
	if day == nil {
		return sessionmessage.SenderRoleRunner, nil
	}
	group, err := s.groupDao.FindByID(ctx, day.GroupID)
	if err != nil {
		return "", fmt.Errorf("error al buscar grupo del día: %w", err)
	}
	if group == nil {
		return sessionmessage.SenderRoleRunner, nil
	}
	team, err := s.teamDao.FindByID(ctx, group.TeamID)
	if err != nil {
		return "", fmt.Errorf("error al buscar equipo del grupo: %w", err)
	}
	if team != nil && team.OwnerID == senderUserID {
		return sessionmessage.SenderRoleTrainer, nil
	}
	return sessionmessage.SenderRoleRunner, nil
}

// toSessionMessageResponse mapea el modelo al shape plano confirmado (D8).
func toSessionMessageResponse(message *dbs.SessionMessage, recipientUserIDs []int64) sessionmessage.SessionMessageResponse {
	if recipientUserIDs == nil {
		recipientUserIDs = []int64{}
	}
	return sessionmessage.SessionMessageResponse{
		ID:                message.ID,
		SessionInstanceID: message.SessionInstanceID,
		SenderUserID:      message.SenderUserID,
		SenderRole:        message.SenderRole,
		Type:              message.Type,
		RecipientMode:     message.RecipientMode,
		RecipientUserIDs:  recipientUserIDs,
		Body:              message.Body,
		ReplyToMessageID:  message.ReplyToMessageID,
		CreatedAt:         message.CreatedAt,
	}
}
