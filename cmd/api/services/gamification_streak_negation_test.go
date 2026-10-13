package services

// --- Negación del trigger de racha por calendario (task 2.4): los writes de
// calendario (cambio de kind incluido) NO escriben nada de gamificación: ni
// unseen ni streak_state. Ningún servicio de calendario tiene acceso al
// trigger/componentes de writes — acá lo pinneamos en runtime con DB real.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestGamificationTrigger_CalendarioKindChangeNoDispara(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "negcal")
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)
	date := time.Now().AddDate(0, 0, 3)

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "rest"})
	require.NoError(t, err)

	// Cambio de kind del MISMO día: write puro de calendario (rest → other).
	other := "gimnasio"
	_, err = svc.UpsertDay(nil, group.ID, owner.ID, date, calendar.CalendarDayRequest{Kind: "other", OtherName: &other})
	require.NoError(t, err)

	var unseenCount int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Count(&unseenCount).Error)
	var stateCount int64
	require.NoError(t, db.Model(&dbs.GamificationStreakState{}).Count(&stateCount).Error)
	assert.Zero(t, unseenCount, "el write de calendario nunca escribe unseen")
	assert.Zero(t, stateCount, "el write de calendario nunca actualiza streak_state")
}
