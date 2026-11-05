package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/sessionmessage"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
)

func TestFullSeederMessages_AreVisibleByModeAndCarryReplyAcrossCalendarDays(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := seedFullPaymentsPrerequisites(t, db)
	require.NoError(t, s.seedPaymentsAndMessages())

	service := services.NewSessionMessageService(
		daos.NewSessionMessageDao(db), daos.NewSessionInstanceDao(db), daos.NewGroupCalendarDayDao(db),
		daos.NewGroupDao(db), daos.NewTeamDao(db),
	)
	todayID := s.ids.sessionInstances["today_a_main"]
	r1Today, err := service.List(nil, s.ids.users["r1"], todayID, 0)
	require.NoError(t, err)
	assertMessageVisible(t, r1Today.Messages, "today_all", sessionmessage.TypeMessageInfo, sessionmessage.RecipientModeAll, []int64{}, sessionmessage.SenderRoleTrainer)
	assertMessageVisible(t, r1Today.Messages, "today_direct_r1", sessionmessage.TypeMessageAviso, sessionmessage.RecipientModeDirect, []int64{s.ids.users["r1"]}, sessionmessage.SenderRoleTrainer)
	assertMessageVisible(t, r1Today.Messages, "today_multiple", sessionmessage.TypeMessageAlerta, sessionmessage.RecipientModeMultiple, []int64{s.ids.users["r1"], s.ids.users["r2"]}, sessionmessage.SenderRoleTrainer)
	assertMessageVisible(t, r1Today.Messages, "today_reply", sessionmessage.TypeMessageInfo, sessionmessage.RecipientModeDirect, []int64{s.ids.users["trainer_1"]}, sessionmessage.SenderRoleRunner)
	assertMessageNotVisible(t, r1Today.Messages, "today_direct_r2")

	r2Today, err := service.List(nil, s.ids.users["r2"], todayID, 0)
	require.NoError(t, err)
	assertMessageVisible(t, r2Today.Messages, "today_direct_r2", sessionmessage.TypeMessageAlerta, sessionmessage.RecipientModeDirect, []int64{s.ids.users["r2"]}, sessionmessage.SenderRoleTrainer)
	assertMessageNotVisible(t, r2Today.Messages, "today_direct_r1")

	pastID := s.ids.sessionInstances["activity_a_secondary_minus7"]
	r4Past, err := service.List(nil, s.ids.users["r4"], pastID, 0)
	require.NoError(t, err)
	assertMessageVisible(t, r4Past.Messages, "past_all", sessionmessage.TypeMessageAviso, sessionmessage.RecipientModeAll, []int64{}, sessionmessage.SenderRoleTrainer)
	assertMessageVisible(t, r4Past.Messages, "past_direct_r4", sessionmessage.TypeMessageInfo, sessionmessage.RecipientModeDirect, []int64{s.ids.users["r4"]}, sessionmessage.SenderRoleTrainer)
	assertMessageVisible(t, r4Past.Messages, "past_multiple", sessionmessage.TypeMessageAlerta, sessionmessage.RecipientModeMultiple, []int64{s.ids.users["r4"], s.ids.users["r5"]}, sessionmessage.SenderRoleTrainer)
	assertMessageVisible(t, r4Past.Messages, "past_reply", sessionmessage.TypeMessageInfo, sessionmessage.RecipientModeDirect, []int64{s.ids.users["trainer_1"]}, sessionmessage.SenderRoleRunner)

	r5Past, err := service.List(nil, s.ids.users["r5"], pastID, 0)
	require.NoError(t, err)
	assertMessageNotVisible(t, r5Past.Messages, "past_direct_r4")
	assertMessageVisible(t, r5Past.Messages, "past_multiple", sessionmessage.TypeMessageAlerta, sessionmessage.RecipientModeMultiple, []int64{s.ids.users["r4"], s.ids.users["r5"]}, sessionmessage.SenderRoleTrainer)

	var recipientRows []dbs.SessionMessageRecipient
	require.NoError(t, db.Order("message_id, user_id").Find(&recipientRows).Error)
	assert.NotEmpty(t, recipientRows)
	var pushTokens []dbs.PushToken
	require.NoError(t, db.Order("user_id").Find(&pushTokens).Error)
	require.Len(t, pushTokens, 4)
	for _, key := range []string{"trainer_1", "r1", "r2", "r3"} {
		var token dbs.PushToken
		require.NoError(t, db.Where("user_id = ?", s.ids.users[key]).Take(&token).Error)
		assert.Contains(t, token.Token, "ExponentPushToken[seed-full-")
	}
}

func assertMessageVisible(t *testing.T, messages []sessionmessage.SessionMessageResponse, key, kind, mode string, recipients []int64, senderRole string) {
	t.Helper()
	var found *sessionmessage.SessionMessageResponse
	for i := range messages {
		if messageHasSeedKey(messages[i].Body, key) {
			found = &messages[i]
			break
		}
	}
	require.NotNil(t, found, "mensaje %s visible", key)
	assert.Equal(t, kind, found.Type)
	assert.Equal(t, mode, found.RecipientMode)
	assert.Equal(t, recipients, found.RecipientUserIDs)
	assert.Equal(t, senderRole, found.SenderRole)
	if strings.HasSuffix(key, "reply") {
		assert.NotNil(t, found.ReplyToMessageID)
	} else {
		assert.Nil(t, found.ReplyToMessageID)
	}
}

func assertMessageNotVisible(t *testing.T, messages []sessionmessage.SessionMessageResponse, key string) {
	t.Helper()
	for _, message := range messages {
		assert.False(t, messageHasSeedKey(message.Body, key), "mensaje %s no debe ser visible", key)
	}
}

func messageHasSeedKey(body, key string) bool {
	return strings.Contains(body, "[seed_full:message:"+key+"]")
}
