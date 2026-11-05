package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/testutils"
)

func TestGap39_AsyncTodayAllowsAllCalendarWrites(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "gap39writes")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "gap39writes")
	svc := presencialWiringSvc(db, true)
	calendarDao := daos.NewGroupCalendarDayDao(db)
	today := time.Now()
	todayStr := today.Format("2006-01-02")

	day, err := svc.UpsertDay(nil, group.ID, owner.ID, today, calendar.CalendarDayRequest{Kind: "training", SessionID: &session.ID})
	require.NoError(t, err)
	require.NotNil(t, day)
	assert.Equal(t, "training", day.Kind)
	assert.NotNil(t, day.SessionInstance)
	stored, err := calendarDao.FindByGroupAndDate(nil, group.ID, today)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.NotNil(t, stored.SessionInstanceID)

	require.NoError(t, svc.DeleteDay(nil, group.ID, owner.ID, today))
	stored, err = calendarDao.FindByGroupAndDate(nil, group.ID, today)
	require.NoError(t, err)
	assert.Nil(t, stored)

	_, err = svc.Bulk(nil, group.ID, owner.ID, calendar.BulkRequest{Dates: []string{todayStr}, Kind: "rest"})
	require.NoError(t, err)

	require.NoError(t, svc.BulkClear(nil, group.ID, owner.ID, calendar.BulkClearRequest{Dates: []string{todayStr}}))
	stored, err = calendarDao.FindByGroupAndDate(nil, group.ID, today)
	require.NoError(t, err)
	assert.Nil(t, stored)

	plan := exclPlan(t, db, owner.ID, "gap39stamp", []string{"rest"}, nil)
	_, err = svc.Stamp(nil, group.ID, owner.ID, calendar.StampRequest{PlanID: plan.ID, StartDate: todayStr})
	require.NoError(t, err)
	stored, err = calendarDao.FindByGroupAndDate(nil, group.ID, today)
	require.NoError(t, err)
	require.NotNil(t, stored)

	_, err = svc.Shift(nil, group.ID, owner.ID, calendar.ShiftRequest{FromDate: todayStr, Days: 1})
	require.NoError(t, err)
	stored, err = calendarDao.FindByGroupAndDate(nil, group.ID, today)
	require.NoError(t, err)
	assert.Nil(t, stored)
	tomorrow, err := calendarDao.FindByGroupAndDate(nil, group.ID, today.AddDate(0, 0, 1))
	require.NoError(t, err)
	assert.NotNil(t, tomorrow)
}

func TestGap39_PresencialTodayStartedStillClosed(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, group := task3OwnerGroup(t, db, "gap39pres")
	session, _ := referenciaCatalogSession(t, db, owner.ID, "gap39pres")
	svc := presencialWiringSvc(db, false)
	today := time.Now()

	_, err := svc.UpsertDay(nil, group.ID, owner.ID, today, presencialRequest(0, 1, &session.ID))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCalendarDayClosed)
	stored, findErr := daos.NewGroupCalendarDayDao(db).FindByGroupAndDate(nil, group.ID, today)
	require.NoError(t, findErr)
	assert.Nil(t, stored)
}
