package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/sessionmessage"
)

// mockSessionInstanceDao acumula los lookups del service de mensajes.
type mockSessionInstanceDao struct {
	createFn            func(ctx *gin.Context, s *dbs.SessionInstance) error
	findByIDFn          func(ctx *gin.Context, id int64) (*dbs.SessionInstance, error)
	deleteFn            func(ctx *gin.Context, id int64) error
	hasFeedbackFn       func(ctx *gin.Context, id int64) (bool, error)
	hasInstanceAccessFn func(ctx *gin.Context, instanceID, callerID int64) (bool, error)
}

func (m *mockSessionInstanceDao) Create(ctx *gin.Context, s *dbs.SessionInstance) error {
	if m.createFn != nil {
		return m.createFn(ctx, s)
	}
	return nil
}

func (m *mockSessionInstanceDao) FindByID(ctx *gin.Context, id int64) (*dbs.SessionInstance, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockSessionInstanceDao) FindByIDs(_ *gin.Context, _ []int64) ([]dbs.SessionInstance, error) {
	return nil, nil
}

func (m *mockSessionInstanceDao) Delete(ctx *gin.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockSessionInstanceDao) HasFeedback(ctx *gin.Context, id int64) (bool, error) {
	if m.hasFeedbackFn != nil {
		return m.hasFeedbackFn(ctx, id)
	}
	return false, nil
}

func (m *mockSessionInstanceDao) HasInstanceAccess(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
	if m.hasInstanceAccessFn != nil {
		return m.hasInstanceAccessFn(ctx, instanceID, callerID)
	}
	return false, nil
}

// mockSessionMessageDao simula la persistencia de mensajes y recipients.
type mockSessionMessageDao struct {
	createFn           func(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error
	findByIDFn         func(ctx *gin.Context, id int64) (*dbs.SessionMessage, []int64, error)
	findVisibleSinceFn func(ctx *gin.Context, sessionInstanceID, viewerUserID, sinceID int64) ([]dbs.SessionMessage, [][]int64, error)
}

func (m *mockSessionMessageDao) Create(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
	if m.createFn != nil {
		return m.createFn(ctx, message, recipientUserIDs)
	}
	return nil
}

func (m *mockSessionMessageDao) FindByID(ctx *gin.Context, id int64) (*dbs.SessionMessage, []int64, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil, nil
}

func (m *mockSessionMessageDao) FindVisibleSince(ctx *gin.Context, sessionInstanceID, viewerUserID, sinceID int64) ([]dbs.SessionMessage, [][]int64, error) {
	if m.findVisibleSinceFn != nil {
		return m.findVisibleSinceFn(ctx, sessionInstanceID, viewerUserID, sinceID)
	}
	return nil, nil, nil
}

// sessionMessageSvcWith arma el service sobre sus cinco DAOs simulados.
func sessionMessageSvcWith(
	messageDao *mockSessionMessageDao,
	instanceDao *mockSessionInstanceDao,
	dayDao *mockGroupCalendarDao,
	groupDao *mockGroupDao,
	teamDao *mockTeamDao,
) SessionMessageServiceInterface {
	return &sessionMessageService{
		sessionMessageDao:  messageDao,
		sessionInstanceDao: instanceDao,
		calendarDayDao:     dayDao,
		groupDao:           groupDao,
		teamDao:            teamDao,
	}
}

func baseInstanceDao(access bool) *mockSessionInstanceDao {
	return &mockSessionInstanceDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.SessionInstance, error) {
			return &dbs.SessionInstance{ID: id}, nil
		},
		hasInstanceAccessFn: func(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
			return access, nil
		},
	}
}

func dayForOwner(owner int64) (*mockGroupCalendarDao, *mockGroupDao, *mockTeamDao) {
	dayDao := &mockGroupCalendarDao{findBySessionInstanceIDFn: func(ctx *gin.Context, sessionInstanceID int64) (*dbs.GroupCalendarDay, error) {
		return &dbs.GroupCalendarDay{ID: 5, GroupID: 3}, nil
	}}
	groupDao := &mockGroupDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
		return &dbs.Group{ID: id, TeamID: 77}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: owner}, nil
	}}
	return dayDao, groupDao, teamDao
}

func newMessageService(access bool, owner int64) SessionMessageServiceInterface {
	messageDao := &mockSessionMessageDao{
		createFn: func(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
			message.ID = 9
			return nil
		},
	}
	dayDao, groupDao, teamDao := dayForOwner(owner)
	return sessionMessageSvcWith(messageDao, baseInstanceDao(access), dayDao, groupDao, teamDao)
}

func validMessageRequest() sessionmessage.SendMessageRequest {
	return sessionmessage.SendMessageRequest{
		Type:          sessionmessage.TypeMessageAviso,
		RecipientMode: sessionmessage.RecipientModeAll,
		Body:          "Falten 5 minutos",
	}
}

func TestSessionMessageService_Create_NotFound(t *testing.T) {
	svc := sessionMessageSvcWith(&mockSessionMessageDao{}, &mockSessionInstanceDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.SessionInstance, error) {
		return nil, nil
	}}, &mockGroupCalendarDao{}, &mockGroupDao{}, &mockTeamDao{})

	_, err := svc.Create(nil, 7, 88, validMessageRequest())

	require.ErrorIs(t, err, ErrCalendarInstanceNotFound)
}

func TestSessionMessageService_Create_Forbidden(t *testing.T) {
	svc := newMessageService(false, 99)

	_, err := svc.Create(nil, 7, 88, validMessageRequest())

	require.ErrorIs(t, err, ErrSessionMessageForbidden)
}

func TestSessionMessageService_Create_SenderRoleTrainer(t *testing.T) {
	var persisted *dbs.SessionMessage
	messageDao := &mockSessionMessageDao{createFn: func(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
		message.ID = 9
		persisted = message
		return nil
	}}
	dayDao, groupDao, teamDao := dayForOwner(7)
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, groupDao, teamDao)

	resp, err := svc.Create(nil, 7, 88, validMessageRequest())

	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Equal(t, sessionmessage.SenderRoleTrainer, persisted.SenderRole)
	assert.Equal(t, sessionmessage.SenderRoleTrainer, resp.SenderRole)
	assert.Equal(t, "trainer", resp.SenderRole)
}

func TestSessionMessageService_Create_SenderRoleRunner(t *testing.T) {
	var persisted *dbs.SessionMessage
	messageDao := &mockSessionMessageDao{createFn: func(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
		message.ID = 9
		persisted = message
		return nil
	}}
	dayDao, groupDao, teamDao := dayForOwner(99)
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, groupDao, teamDao)

	resp, err := svc.Create(nil, 7, 88, validMessageRequest())

	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Equal(t, sessionmessage.SenderRoleRunner, persisted.SenderRole)
	assert.Equal(t, "runner", resp.SenderRole)
}

func TestSessionMessageService_Create_SenderRoleRunnerWithoutDay(t *testing.T) {
	messageDao := &mockSessionMessageDao{}
	dayDao := &mockGroupCalendarDao{findBySessionInstanceIDFn: func(ctx *gin.Context, sessionInstanceID int64) (*dbs.GroupCalendarDay, error) {
		return nil, nil
	}}
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, &mockGroupDao{}, &mockTeamDao{})

	resp, err := svc.Create(nil, 7, 88, validMessageRequest())

	require.NoError(t, err)
	assert.Equal(t, "runner", resp.SenderRole)
}

func TestSessionMessageService_Create_ValidationErrors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(req *sessionmessage.SendMessageRequest)
	}{
		{"type invalido", func(r *sessionmessage.SendMessageRequest) { r.Type = "urgente" }},
		{"recipient_mode invalido", func(r *sessionmessage.SendMessageRequest) { r.RecipientMode = "grupo" }},
		{"all con destinatarios", func(r *sessionmessage.SendMessageRequest) { r.RecipientUserIDs = []int64{7} }},
		{"direct sin destinatarios", func(r *sessionmessage.SendMessageRequest) { r.RecipientMode = sessionmessage.RecipientModeDirect }},
		{"direct con dos", func(r *sessionmessage.SendMessageRequest) {
			r.RecipientMode = sessionmessage.RecipientModeDirect
			r.RecipientUserIDs = []int64{7, 12}
		}},
		{"multiple con solo uno", func(r *sessionmessage.SendMessageRequest) {
			r.RecipientMode = sessionmessage.RecipientModeMultiple
			r.RecipientUserIDs = []int64{7}
		}},
		{"multiple con duplicados", func(r *sessionmessage.SendMessageRequest) {
			r.RecipientMode = sessionmessage.RecipientModeMultiple
			r.RecipientUserIDs = []int64{7, 7}
		}},
		{"multiple con duplicado intercalado", func(r *sessionmessage.SendMessageRequest) {
			r.RecipientMode = sessionmessage.RecipientModeMultiple
			r.RecipientUserIDs = []int64{7, 12, 7}
		}},
		{"body vacio tras trim", func(r *sessionmessage.SendMessageRequest) { r.Body = "   " }},
		{"body demasiado largo", func(r *sessionmessage.SendMessageRequest) { r.Body = strings.Repeat("a", maxBodyLen+1) }},
	}
	for _, tc := range cases {
		req := validMessageRequest()
		tc.mutate(&req)
		svc := newMessageService(true, 99)

		_, err := svc.Create(nil, 7, 88, req)

		require.ErrorIs(t, err, ErrSessionMessageInvalid, tc.name)
	}
}

func TestSessionMessageService_Create_RecipientWithoutAccess(t *testing.T) {
	var accessDenied bool
	instanceDao := baseInstanceDao(true)
	instanceDao.hasInstanceAccessFn = func(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
		accessDenied = callerID == 12
		return !accessDenied, nil
	}
	dayDao, groupDao, teamDao := dayForOwner(99)
	svc := sessionMessageSvcWith(&mockSessionMessageDao{}, instanceDao, dayDao, groupDao, teamDao)
	req := sessionmessage.SendMessageRequest{
		Type:             sessionmessage.TypeMessageInfo,
		RecipientMode:    sessionmessage.RecipientModeDirect,
		RecipientUserIDs: []int64{12},
		Body:             "che",
	}

	_, err := svc.Create(nil, 7, 88, req)

	require.ErrorIs(t, err, ErrSessionMessageInvalid)
}

func TestSessionMessageService_Create_DuplicateRecipientsNotPersisted(t *testing.T) {
	persisted := false
	messageDao := &mockSessionMessageDao{createFn: func(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
		persisted = true
		message.ID = 9
		return nil
	}}
	dayDao, groupDao, teamDao := dayForOwner(99)
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, groupDao, teamDao)
	req := validMessageRequest()
	req.RecipientMode = sessionmessage.RecipientModeMultiple
	req.RecipientUserIDs = []int64{7, 7}

	_, err := svc.Create(nil, 7, 88, req)

	require.ErrorIs(t, err, ErrSessionMessageInvalid)
	assert.Contains(t, err.Error(), "duplicados")
	assert.False(t, persisted)
}

func TestSessionMessageService_Create_DirectSuccess(t *testing.T) {
	var gotRecipients []int64
	messageDao := &mockSessionMessageDao{createFn: func(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
		gotRecipients = recipientUserIDs
		message.ID = 9
		return nil
	}}
	dayDao, groupDao, teamDao := dayForOwner(99)
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, groupDao, teamDao)
	created := time.Now().UTC()
	req := sessionmessage.SendMessageRequest{
		Type:             sessionmessage.TypeMessageInfo,
		RecipientMode:    sessionmessage.RecipientModeDirect,
		RecipientUserIDs: []int64{12},
		Body:             "  movete ya  ",
	}

	resp, err := svc.Create(nil, 7, 88, req)

	require.NoError(t, err)
	assert.Equal(t, []int64{12}, gotRecipients)
	assert.Equal(t, "movete ya", resp.Body)
	assert.Equal(t, "direct", resp.RecipientMode)
	assert.Equal(t, []int64{12}, resp.RecipientUserIDs)
	assert.Equal(t, int64(9), resp.ID)
	assert.Equal(t, int64(88), resp.SessionInstanceID)
	assert.Equal(t, int64(7), resp.SenderUserID)
	assert.Nil(t, resp.ReplyToMessageID)
	_ = created
}

func TestSessionMessageService_Create_AllReturnsEmptyRecipientList(t *testing.T) {
	var gotRecipients []int64
	messageDao := &mockSessionMessageDao{createFn: func(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
		gotRecipients = recipientUserIDs
		message.ID = 9
		return nil
	}}
	dayDao, groupDao, teamDao := dayForOwner(99)
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, groupDao, teamDao)

	resp, err := svc.Create(nil, 7, 88, validMessageRequest())

	require.NoError(t, err)
	assert.Empty(t, gotRecipients)
	assert.NotNil(t, resp.RecipientUserIDs)
	assert.Len(t, resp.RecipientUserIDs, 0)
}

func TestSessionMessageService_Create_ReplyValidations(t *testing.T) {
	replyTo := int64(5)
	cases := []struct {
		name string
		dao  *mockSessionMessageDao
	}{
		{"id inexistente", &mockSessionMessageDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.SessionMessage, []int64, error) {
			return nil, nil, nil
		}}},
		{"otra sesion", &mockSessionMessageDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.SessionMessage, []int64, error) {
			return &dbs.SessionMessage{ID: id, SessionInstanceID: 99, RecipientMode: sessionmessage.RecipientModeAll}, nil, nil
		}}},
		{"no visible", &mockSessionMessageDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.SessionMessage, []int64, error) {
			return &dbs.SessionMessage{ID: id, SessionInstanceID: 88, SenderUserID: 13, RecipientMode: sessionmessage.RecipientModeMultiple}, []int64{12}, nil
		}}},
	}
	for _, tc := range cases {
		req := validMessageRequest()
		req.ReplyToMessageID = &replyTo
		dayDao, groupDao, teamDao := dayForOwner(99)
		svc := sessionMessageSvcWith(tc.dao, baseInstanceDao(true), dayDao, groupDao, teamDao)

		_, err := svc.Create(nil, 7, 88, req)

		require.ErrorIs(t, err, ErrSessionMessageInvalid, tc.name)
	}
}

func TestSessionMessageService_Create_ReplyVisibleAsRecipient(t *testing.T) {
	replyTo := int64(5)
	messageDao := &mockSessionMessageDao{
		createFn: func(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
			message.ID = 9
			return nil
		},
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.SessionMessage, []int64, error) {
			return &dbs.SessionMessage{ID: id, SessionInstanceID: 88, SenderUserID: 13, RecipientMode: sessionmessage.RecipientModeMultiple}, []int64{7}, nil
		},
	}
	dayDao, groupDao, teamDao := dayForOwner(99)
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, groupDao, teamDao)
	req := validMessageRequest()
	req.ReplyToMessageID = &replyTo

	resp, err := svc.Create(nil, 7, 88, req)

	require.NoError(t, err)
	require.NotNil(t, resp.ReplyToMessageID)
	assert.Equal(t, replyTo, *resp.ReplyToMessageID)
}

func TestSessionMessageService_List_DelegatesToDaoAndMaps(t *testing.T) {
	var gotSince int64
	messageDao := &mockSessionMessageDao{findVisibleSinceFn: func(ctx *gin.Context, sessionInstanceID, viewerUserID, sinceID int64) ([]dbs.SessionMessage, [][]int64, error) {
		gotSince = sinceID
		createdAt := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
		return []dbs.SessionMessage{
			{ID: 9, SessionInstanceID: 88, SenderUserID: 7, SenderRole: sessionmessage.SenderRoleTrainer, Type: sessionmessage.TypeMessageAviso, RecipientMode: sessionmessage.RecipientModeMultiple, Body: "uno", CreatedAt: createdAt},
			{ID: 10, SessionInstanceID: 88, SenderUserID: 13, SenderRole: sessionmessage.SenderRoleRunner, Type: sessionmessage.TypeMessageInfo, RecipientMode: sessionmessage.RecipientModeAll, Body: "dos", CreatedAt: createdAt},
		}, [][]int64{{12, 7}, {}}, nil
	}}
	dayDao, groupDao, teamDao := dayForOwner(99)
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, groupDao, teamDao)

	resp, err := svc.List(nil, 7, 88, 9)

	require.NoError(t, err)
	assert.Equal(t, int64(9), gotSince)
	require.Len(t, resp.Messages, 2)
	assert.Equal(t, int64(9), resp.Messages[0].ID)
	assert.Equal(t, []int64{12, 7}, resp.Messages[0].RecipientUserIDs)
	assert.Equal(t, "aviso", resp.Messages[0].Type)
	assert.Equal(t, []int64{}, resp.Messages[1].RecipientUserIDs, "all debe devolver [] sin omitempty")
	_, hasNullRecipients := any(resp.Messages[1].RecipientUserIDs).(int64)
	assert.False(t, hasNullRecipients)
}

func TestSessionMessageService_List_NotFoundAndForbidden(t *testing.T) {
	notFound := sessionMessageSvcWith(&mockSessionMessageDao{}, &mockSessionInstanceDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.SessionInstance, error) {
		return nil, nil
	}}, &mockGroupCalendarDao{}, &mockGroupDao{}, &mockTeamDao{})
	_, err := notFound.List(nil, 7, 88, 0)
	require.ErrorIs(t, err, ErrCalendarInstanceNotFound)

	forbidden := sessionMessageSvcWith(&mockSessionMessageDao{}, baseInstanceDao(false), &mockGroupCalendarDao{}, &mockGroupDao{}, &mockTeamDao{})
	_, err = forbidden.List(nil, 7, 88, 0)
	require.ErrorIs(t, err, ErrSessionMessageForbidden)
}

func TestSessionMessageService_List_SinceZeroAsksFullHistory(t *testing.T) {
	var gotSince int64
	messageDao := &mockSessionMessageDao{findVisibleSinceFn: func(ctx *gin.Context, sessionInstanceID, viewerUserID, sinceID int64) ([]dbs.SessionMessage, [][]int64, error) {
		gotSince = sinceID
		return nil, nil, nil
	}}
	dayDao, groupDao, teamDao := dayForOwner(99)
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, groupDao, teamDao)

	resp, err := svc.List(nil, 7, 88, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(0), gotSince)
	assert.NotNil(t, resp.Messages)
	assert.Len(t, resp.Messages, 0)
}

func TestSessionMessageService_Create_DaoErrorPropagates(t *testing.T) {
	boom := errors.New("boom db")
	messageDao := &mockSessionMessageDao{createFn: func(ctx *gin.Context, message *dbs.SessionMessage, recipientUserIDs []int64) error {
		return boom
	}}
	dayDao, groupDao, teamDao := dayForOwner(99)
	svc := sessionMessageSvcWith(messageDao, baseInstanceDao(true), dayDao, groupDao, teamDao)

	_, err := svc.Create(nil, 7, 88, validMessageRequest())

	require.Error(t, err)
	require.ErrorIs(t, err, boom)
	assert.False(t, errors.Is(err, ErrSessionMessageInvalid))
}

var _ daos.SessionMessageDaoInterface = (*mockSessionMessageDao)(nil)
var _ daos.SessionInstanceDaoInterface = (*mockSessionInstanceDao)(nil)
