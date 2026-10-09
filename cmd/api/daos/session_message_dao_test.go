package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestSessionMessageDao_ImplementsInterface(t *testing.T) {
	dao := NewSessionMessageDao(&gorm.DB{})
	var iface SessionMessageDaoInterface = dao
	_ = iface
}

func TestSessionMessageDao_Create_AllMode_NoRecipients(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	inst := &dbs.SessionInstance{Name: "Inst all"}
	require.NoError(t, db.Create(inst).Error)

	msg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: 1, SenderRole: "trainer", Type: "info", RecipientMode: "all", Body: "para todos"}
	require.NoError(t, dao.Create(nil, msg, nil))
	require.Greater(t, msg.ID, int64(0))

	var count int64
	require.NoError(t, db.Model(&dbs.SessionMessageRecipient{}).Where("message_id = ?", msg.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestSessionMessageDao_Create_WithRecipients_PersistsRows(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	inst := &dbs.SessionInstance{Name: "Inst multiple"}
	require.NoError(t, db.Create(inst).Error)
	recipient := persistUser(db, "msg-recipient@test.com", "20100001")
	recipient2 := persistUser(db, "msg-recipient-2@test.com", "20100002")

	msg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: 1, SenderRole: "trainer", Type: "aviso", RecipientMode: "multiple", Body: "para vos"}
	require.NoError(t, dao.Create(nil, msg, []int64{recipient.ID, recipient2.ID}))

	var rows []dbs.SessionMessageRecipient
	require.NoError(t, db.Where("message_id = ?", msg.ID).Order("user_id ASC").Find(&rows).Error)
	require.Len(t, rows, 2)
	assert.Equal(t, recipient.ID, rows[0].UserID)
	assert.Equal(t, recipient2.ID, rows[1].UserID)
}

func TestSessionMessageDao_Create_Atomic_RollbackOnRecipientsError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	inst := &dbs.SessionInstance{Name: "Inst atomica"}
	require.NoError(t, db.Create(inst).Error)

	// user_id repetido viola la PK compuesta → la tx completa se revierte.
	msg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: 1, SenderRole: "trainer", Type: "info", RecipientMode: "multiple", Body: "no debe quedar"}
	require.Error(t, dao.Create(nil, msg, []int64{7, 7}))

	found, _, err := dao.FindByID(nil, msg.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSessionMessageDao_FindByID_Found_WithRecipients(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	inst := &dbs.SessionInstance{Name: "Inst find"}
	require.NoError(t, db.Create(inst).Error)
	r1 := persistUser(db, "msg-find-r1@test.com", "20100010")
	r2 := persistUser(db, "msg-find-r2@test.com", "20100011")

	msg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: 1, SenderRole: "trainer", Type: "aviso", RecipientMode: "multiple", Body: "detalle inquire"}
	require.NoError(t, dao.Create(nil, msg, []int64{r2.ID, r1.ID}))

	found, userIDs, err := dao.FindByID(nil, msg.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, inst.ID, found.SessionInstanceID)
	assert.Equal(t, r1.ID, userIDs[0])
	assert.Equal(t, r2.ID, userIDs[1])
}

func TestSessionMessageDao_FindByID_NotFound_ReturnsNil(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)

	found, userIDs, err := dao.FindByID(nil, 999999)
	require.NoError(t, err)
	assert.Nil(t, found)
	assert.Nil(t, userIDs)
}

func TestSessionMessageDao_Roundtrip_Fields(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	inst := &dbs.SessionInstance{Name: "Inst roundtrip"}
	require.NoError(t, db.Create(inst).Error)

	replied := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: 1, SenderRole: "trainer", Type: "info", RecipientMode: "all", Body: "base"}
	require.NoError(t, dao.Create(nil, replied, nil))

	msg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: 2, SenderRole: "runner", Type: "alerta", RecipientMode: "all", Body: "respuesta con tilde y ñ", ReplyToMessageID: &replied.ID}
	require.NoError(t, dao.Create(nil, msg, nil))

	found, _, err := dao.FindByID(nil, msg.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, msg.SessionInstanceID, found.SessionInstanceID)
	assert.Equal(t, int64(2), found.SenderUserID)
	assert.Equal(t, "runner", found.SenderRole)
	assert.Equal(t, "alerta", found.Type)
	assert.Equal(t, "all", found.RecipientMode)
	assert.Equal(t, "respuesta con tilde y ñ", found.Body)
	require.NotNil(t, found.ReplyToMessageID)
	assert.Equal(t, replied.ID, *found.ReplyToMessageID)
	assert.False(t, found.CreatedAt.IsZero())
}

func TestSessionMessageDao_FindVisibleSince_Visibility(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	inst := &dbs.SessionInstance{Name: "Inst visibilidad"}
	require.NoError(t, db.Create(inst).Error)
	trainer := persistUser(db, "msg-vis-trainer@test.com", "20100020")
	runner := persistUser(db, "msg-vis-runner@test.com", "20100021")
	recipient := persistUser(db, "msg-vis-recipient@test.com", "20100022")
	outsider := persistUser(db, "msg-vis-outsider@test.com", "20100023")

	allMsg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: trainer.ID, SenderRole: "trainer", Type: "info", RecipientMode: "all", Body: "all"}
	require.NoError(t, dao.Create(nil, allMsg, nil))
	dmMsg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: trainer.ID, SenderRole: "trainer", Type: "aviso", RecipientMode: "direct", Body: "solo para vos"}
	require.NoError(t, dao.Create(nil, dmMsg, []int64{runner.ID}))
	multiMsg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: trainer.ID, SenderRole: "trainer", Type: "alerta", RecipientMode: "multiple", Body: "a la lista"}
	require.NoError(t, dao.Create(nil, multiMsg, []int64{runner.ID, recipient.ID}))

	// all: lo ve el emisor y cualquier user (todos son destinatarios).
	allForOutsider, _, err := dao.FindVisibleSince(nil, inst.ID, outsider.ID, 0)
	require.NoError(t, err)
	require.Len(t, allForOutsider, 1)
	assert.Equal(t, allMsg.ID, allForOutsider[0].ID)

	// direct: lo ven solo emisor y destinatario; el entrenador (sender) lo ve, tercer trainer NO.
	trainerView, _, err := dao.FindVisibleSince(nil, inst.ID, trainer.ID, 0)
	require.NoError(t, err)
	require.Len(t, trainerView, 3) // envió los 3
	assert.Equal(t, []int64{allMsg.ID, dmMsg.ID, multiMsg.ID}, []int64{trainerView[0].ID, trainerView[1].ID, trainerView[2].ID})

	otherTrainer := persistUser(db, "msg-vis-trainer2@test.com", "20100024")
	trainer2View, _, err := dao.FindVisibleSince(nil, inst.ID, otherTrainer.ID, 0)
	require.NoError(t, err)
	require.Len(t, trainer2View, 1) // solo el all
	assert.Equal(t, allMsg.ID, trainer2View[0].ID)

	runnerView, _, err := dao.FindVisibleSince(nil, inst.ID, runner.ID, 0)
	require.NoError(t, err)
	require.Len(t, runnerView, 3) // all + directo + múltiple
	assert.Equal(t, []int64{allMsg.ID, dmMsg.ID, multiMsg.ID}, []int64{runnerView[0].ID, runnerView[1].ID, runnerView[2].ID})

	recipientView, _, err := dao.FindVisibleSince(nil, inst.ID, recipient.ID, 0)
	require.NoError(t, err)
	require.Len(t, recipientView, 2) // all + múltiple
	assert.Equal(t, []int64{allMsg.ID, multiMsg.ID}, []int64{recipientView[0].ID, recipientView[1].ID})

	// outsider: ve el all, NO el direct.
	outsiderView, _, err := dao.FindVisibleSince(nil, inst.ID, outsider.ID, 0)
	require.NoError(t, err)
	require.Len(t, outsiderView, 1)
	assert.Equal(t, allMsg.ID, outsiderView[0].ID)
}

// DM en el sentido runner→entrenador: el corredor envía un direct al entrenador;
// el entrenador (destinatario) lo ve, otro corredor del contexto NO.
func TestSessionMessageDao_FindVisibleSince_DMRunnerToCoach(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	inst := &dbs.SessionInstance{Name: "Inst dm runner a coach"}
	require.NoError(t, db.Create(inst).Error)
	runner := persistUser(db, "msg-dm-runner@test.com", "20100040")
	trainer := persistUser(db, "msg-dm-trainer@test.com", "20100041")
	otherRunner := persistUser(db, "msg-dm-runner-2@test.com", "20100042")

	dmMsg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: runner.ID, SenderRole: "runner", Type: "aviso", RecipientMode: "direct", Body: "coach, duda con la serie"}
	require.NoError(t, dao.Create(nil, dmMsg, []int64{trainer.ID}))

	// sender ve su propio mensaje
	runnerView, _, err := dao.FindVisibleSince(nil, inst.ID, runner.ID, 0)
	require.NoError(t, err)
	require.Len(t, runnerView, 1)
	assert.Equal(t, dmMsg.ID, runnerView[0].ID)

	// el destinatario entrenador lo ve
	trainerView, _, err := dao.FindVisibleSince(nil, inst.ID, trainer.ID, 0)
	require.NoError(t, err)
	require.Len(t, trainerView, 1)
	assert.Equal(t, dmMsg.ID, trainerView[0].ID)

	// otro corredor NO lo ve (privacidad DM)
	otherRunnerView, _, err := dao.FindVisibleSince(nil, inst.ID, otherRunner.ID, 0)
	require.NoError(t, err)
	require.Len(t, otherRunnerView, 0)
}

func TestSessionMessageDao_FindVisibleSince_MultipleIsolatedBetweenSessions(t *testing.T) {	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	instA := &dbs.SessionInstance{Name: "Inst A"}
	instB := &dbs.SessionInstance{Name: "Inst B"}
	require.NoError(t, db.Create(instA).Error)
	require.NoError(t, db.Create(instB).Error)

	msgA := &dbs.SessionMessage{SessionInstanceID: instA.ID, SenderUserID: 1, SenderRole: "trainer", Type: "info", RecipientMode: "all", Body: "de A"}
	require.NoError(t, dao.Create(nil, msgA, nil))
	msgB := &dbs.SessionMessage{SessionInstanceID: instB.ID, SenderUserID: 1, SenderRole: "trainer", Type: "info", RecipientMode: "all", Body: "de B"}
	require.NoError(t, dao.Create(nil, msgB, nil))

	messages, _, err := dao.FindVisibleSince(nil, instB.ID, 2, 0)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, msgB.ID, messages[0].ID)
}

func TestSessionMessageDao_FindVisibleSince_CatchUpSince(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	inst := &dbs.SessionInstance{Name: "Inst catchup"}
	require.NoError(t, db.Create(inst).Error)

	var msgIDs []int64
	for _, body := range []string{"uno", "dos", "tres"} {
		msg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: 1, SenderRole: "trainer", Type: "info", RecipientMode: "all", Body: body}
		require.NoError(t, dao.Create(nil, msg, nil))
		msgIDs = append(msgIDs, msg.ID)
	}

	messages, recipientIDs, err := dao.FindVisibleSince(nil, inst.ID, 2, msgIDs[1])
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, msgIDs[2], messages[0].ID)
	require.Len(t, recipientIDs, 1)
	assert.Empty(t, recipientIDs[0]) // all → sin recipient_ids

	messages, recipientIDs, err = dao.FindVisibleSince(nil, inst.ID, 2, 0)
	require.NoError(t, err)
	require.Len(t, messages, 3)
	assert.Equal(t, msgIDs, []int64{messages[0].ID, messages[1].ID, messages[2].ID})
	for _, ids := range recipientIDs {
		assert.Empty(t, ids)
	}
}

func TestSessionMessageDao_FindVisibleSince_RecipientIDsResolved(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionMessageDao(db)
	inst := &dbs.SessionInstance{Name: "Inst ids"}
	require.NoError(t, db.Create(inst).Error)
	r1 := persistUser(db, "msg-ids-r1@test.com", "20100030")
	r2 := persistUser(db, "msg-ids-r2@test.com", "20100031")

	msg := &dbs.SessionMessage{SessionInstanceID: inst.ID, SenderUserID: 1, SenderRole: "trainer", Type: "info", RecipientMode: "multiple", Body: "lista"}
	require.NoError(t, dao.Create(nil, msg, []int64{r2.ID, r1.ID}))

	messages, recipientIDs, err := dao.FindVisibleSince(nil, inst.ID, r1.ID, 0)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Len(t, recipientIDs[0], 2)
	assert.Equal(t, r1.ID, recipientIDs[0][0])
	assert.Equal(t, r2.ID, recipientIDs[0][1])
}
