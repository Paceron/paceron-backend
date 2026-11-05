package main

import (
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/sessionmessage"
)

type fullSessionMessageSpec struct {
	key, instanceKey, senderKey, senderRole, kind, mode, body string
	recipients                                                []string
	replyToKey                                                string
	createdAt                                                 time.Time
}

func (s *fullSeeder) seedFullMessages(tx *gorm.DB) (map[string]int64, map[string]int64, error) {
	messageIDs := make(map[string]int64)
	for _, spec := range s.fullSessionMessageSpecs() {
		instanceID := s.ids.sessionInstances[spec.instanceKey]
		if err := validateFullTrainerCalendarInstance(tx, instanceID, s.ids.users["trainer_1"]); err != nil {
			return nil, nil, fmt.Errorf("instancia %s de mensajería: %w", spec.instanceKey, err)
		}
		var replyTo *int64
		if spec.replyToKey != "" {
			id := messageIDs[spec.replyToKey]
			if id <= 0 {
				return nil, nil, fmt.Errorf("el reply %s no encuentra el mensaje base %s", spec.key, spec.replyToKey)
			}
			replyTo = &id
		}
		recipientIDs := make([]int64, 0, len(spec.recipients))
		for _, key := range spec.recipients {
			if s.ids.users[key] <= 0 {
				return nil, nil, fmt.Errorf("mensaje %s requiere usuario %s", spec.key, key)
			}
			recipientIDs = append(recipientIDs, s.ids.users[key])
		}
		message, err := ensureFullSessionMessage(tx, dbs.SessionMessage{
			SessionInstanceID: instanceID, SenderUserID: s.ids.users[spec.senderKey],
			SenderRole: spec.senderRole, Type: spec.kind, RecipientMode: spec.mode,
			Body: spec.body, ReplyToMessageID: replyTo, CreatedAt: spec.createdAt,
		}, recipientIDs)
		if err != nil {
			return nil, nil, fmt.Errorf("mensaje %s: %w", spec.key, err)
		}
		messageIDs[spec.key] = message.ID
	}

	pushTokenIDs := make(map[string]int64)
	for _, key := range []string{"trainer_1", "r1", "r2", "r3"} {
		platform := "android"
		if key == "r2" || key == "r3" {
			platform = "web"
		}
		token := "ExponentPushToken[seed-full-" + key + "]"
		row, err := ensureFullPushToken(tx, s.ids.users[key], token, platform, s.now)
		if err != nil {
			return nil, nil, fmt.Errorf("push token de %s: %w", key, err)
		}
		pushTokenIDs[key] = row.ID
	}
	return messageIDs, pushTokenIDs, nil
}

func (s *fullSeeder) fullSessionMessageSpecs() []fullSessionMessageSpec {
	return []fullSessionMessageSpec{
		{key: "today_all", instanceKey: "today_a_main", senderKey: "trainer_1", senderRole: sessionmessage.SenderRoleTrainer,
			kind: sessionmessage.TypeMessageInfo, mode: sessionmessage.RecipientModeAll,
			body: fullMessageBody("today_all", "El entrenamiento de hoy ya está abierto."), createdAt: s.now.Add(-20 * time.Minute)},
		{key: "today_direct_r1", instanceKey: "today_a_main", senderKey: "trainer_1", senderRole: sessionmessage.SenderRoleTrainer,
			kind: sessionmessage.TypeMessageAviso, mode: sessionmessage.RecipientModeDirect, recipients: []string{"r1"},
			body: fullMessageBody("today_direct_r1", "Recordá llevar agua."), createdAt: s.now.Add(-18 * time.Minute)},
		{key: "today_direct_r2", instanceKey: "today_a_main", senderKey: "trainer_1", senderRole: sessionmessage.SenderRoleTrainer,
			kind: sessionmessage.TypeMessageAlerta, mode: sessionmessage.RecipientModeDirect, recipients: []string{"r2"},
			body: fullMessageBody("today_direct_r2", "Revisá la molestia antes de iniciar."), createdAt: s.now.Add(-16 * time.Minute)},
		{key: "today_multiple", instanceKey: "today_a_main", senderKey: "trainer_1", senderRole: sessionmessage.SenderRoleTrainer,
			kind: sessionmessage.TypeMessageAlerta, mode: sessionmessage.RecipientModeMultiple, recipients: []string{"r1", "r2"},
			body: fullMessageBody("today_multiple", "El bloque principal se ajusta por el clima."), createdAt: s.now.Add(-14 * time.Minute)},
		{key: "today_reply", instanceKey: "today_a_main", senderKey: "r1", senderRole: sessionmessage.SenderRoleRunner,
			kind: sessionmessage.TypeMessageInfo, mode: sessionmessage.RecipientModeDirect, recipients: []string{"trainer_1"}, replyToKey: "today_all",
			body: fullMessageBody("today_reply", "Perfecto, ya estoy en camino."), createdAt: s.now.Add(-12 * time.Minute)},
		{key: "past_all", instanceKey: "activity_a_secondary_minus7", senderKey: "trainer_1", senderRole: sessionmessage.SenderRoleTrainer,
			kind: sessionmessage.TypeMessageAviso, mode: sessionmessage.RecipientModeAll,
			body: fullMessageBody("past_all", "Buen trabajo en la sesión de la semana pasada."), createdAt: fullDate(s.now.AddDate(0, 0, -7)).Add(10 * time.Hour)},
		{key: "past_direct_r4", instanceKey: "activity_a_secondary_minus7", senderKey: "trainer_1", senderRole: sessionmessage.SenderRoleTrainer,
			kind: sessionmessage.TypeMessageInfo, mode: sessionmessage.RecipientModeDirect, recipients: []string{"r4"},
			body: fullMessageBody("past_direct_r4", "Te dejé una nota sobre la técnica."), createdAt: fullDate(s.now.AddDate(0, 0, -7)).Add(10*time.Hour + time.Minute)},
		{key: "past_multiple", instanceKey: "activity_a_secondary_minus7", senderKey: "trainer_1", senderRole: sessionmessage.SenderRoleTrainer,
			kind: sessionmessage.TypeMessageAlerta, mode: sessionmessage.RecipientModeMultiple, recipients: []string{"r4", "r5"},
			body: fullMessageBody("past_multiple", "La próxima semana mantenemos el ritmo suave."), createdAt: fullDate(s.now.AddDate(0, 0, -7)).Add(10*time.Hour + 2*time.Minute)},
		{key: "past_reply", instanceKey: "activity_a_secondary_minus7", senderKey: "r4", senderRole: sessionmessage.SenderRoleRunner,
			kind: sessionmessage.TypeMessageInfo, mode: sessionmessage.RecipientModeDirect, recipients: []string{"trainer_1"}, replyToKey: "past_all",
			body: fullMessageBody("past_reply", "Gracias, la sesión salió muy bien."), createdAt: fullDate(s.now.AddDate(0, 0, -7)).Add(10*time.Hour + 3*time.Minute)},
	}
}

func fullMessageBody(key, text string) string {
	return fullSeedMarker + " [seed_full:message:" + key + "] " + text
}

func validateFullTrainerCalendarInstance(tx *gorm.DB, instanceID, trainerID int64) error {
	var days []dbs.GroupCalendarDay
	if err := tx.Where("session_instance_id = ?", instanceID).Find(&days).Error; err != nil {
		return err
	}
	if len(days) != 1 {
		return fmt.Errorf("se esperaba un día de calendario para la instancia, hay %d", len(days))
	}
	var group dbs.Group
	if err := tx.First(&group, days[0].GroupID).Error; err != nil {
		return err
	}
	var team dbs.Team
	if err := tx.First(&team, group.TeamID).Error; err != nil {
		return err
	}
	if team.OwnerID != trainerID || !containsFullSeedMarker(team.Description) {
		return fmt.Errorf("el entrenador no es owner del equipo del día de calendario")
	}
	return nil
}

func ensureFullSessionMessage(tx *gorm.DB, expected dbs.SessionMessage, recipientIDs []int64) (*dbs.SessionMessage, error) {
	var rows []dbs.SessionMessage
	if err := tx.Where("body = ?", expected.Body).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 1 {
		return nil, fmt.Errorf("marker de cuerpo duplicado")
	}
	if len(rows) == 0 {
		if err := tx.Create(&expected).Error; err != nil {
			return nil, err
		}
		if len(recipientIDs) > 0 {
			recipients := make([]dbs.SessionMessageRecipient, 0, len(recipientIDs))
			for _, userID := range recipientIDs {
				recipients = append(recipients, dbs.SessionMessageRecipient{MessageID: expected.ID, UserID: userID})
			}
			if err := tx.Create(&recipients).Error; err != nil {
				return nil, err
			}
		}
		if err := validateFullMessageRecipients(tx, expected.ID, recipientIDs); err != nil {
			return nil, err
		}
		return &expected, nil
	}
	row := rows[0]
	if row.SessionInstanceID != expected.SessionInstanceID || row.SenderUserID != expected.SenderUserID ||
		row.SenderRole != expected.SenderRole || row.Type != expected.Type || row.RecipientMode != expected.RecipientMode ||
		row.Body != expected.Body || !sameInt64Pointer(row.ReplyToMessageID, expected.ReplyToMessageID) {
		return nil, fmt.Errorf("mensaje existente no coincide con la identidad full")
	}
	if err := validateFullMessageRecipients(tx, row.ID, recipientIDs); err != nil {
		return nil, err
	}
	return &row, nil
}

func validateFullMessageRecipients(tx *gorm.DB, messageID int64, expected []int64) error {
	var rows []dbs.SessionMessageRecipient
	if err := tx.Where("message_id = ?", messageID).Order("user_id").Find(&rows).Error; err != nil {
		return err
	}
	actual := make([]int64, len(rows))
	for i := range rows {
		actual[i] = rows[i].UserID
	}
	want := append([]int64(nil), expected...)
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if !equalInt64Slices(actual, want) {
		return fmt.Errorf("recipients existentes no coinciden con el modo del mensaje")
	}
	return nil
}

func ensureFullPushToken(tx *gorm.DB, userID int64, token, platform string, now time.Time) (*dbs.PushToken, error) {
	var rows []dbs.PushToken
	if err := tx.Where("token = ?", token).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 1 {
		return nil, fmt.Errorf("token sintético duplicado")
	}
	if len(rows) == 0 {
		row := dbs.PushToken{UserID: userID, Token: token, Platform: platform, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return nil, err
		}
		return &row, nil
	}
	row := rows[0]
	if row.UserID != userID || row.Token != token || row.Platform != platform {
		return nil, fmt.Errorf("token existente no coincide con el perfil full")
	}
	return &row, nil
}
