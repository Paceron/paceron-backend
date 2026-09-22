package services

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"simple-arq-golang/cmd/api/domains/calendar"
)

func TestNewCalendarTrainingWithoutInstanceError_NilDatesDevuelveSentinel(t *testing.T) {
	err := newCalendarTrainingWithoutInstanceError(nil)
	require.ErrorIs(t, err, ErrCalendarTrainingWithoutInstance)
	require.Equal(t, ErrCalendarTrainingWithoutInstance.Error(), err.Error())
}

func TestNewCalendarTrainingWithoutInstanceError_ListaFechas(t *testing.T) {
	err := newCalendarTrainingWithoutInstanceError([]string{"2026-03-01", "2026-03-02"})
	require.ErrorIs(t, err, ErrCalendarTrainingWithoutInstance)
	require.Equal(t, ErrCalendarTrainingWithoutInstance.Error()+": 2026-03-01, 2026-03-02", err.Error())
}

func TestNewCalendarPresencialCollisionError_NilConflictsDevuelveSentinel(t *testing.T) {
	err := newCalendarPresencialCollisionError(nil)
	require.ErrorIs(t, err, ErrCalendarPresencialCollision)
	require.Equal(t, ErrCalendarPresencialCollision.Error(), err.Error())
}

func TestNewCalendarPresencialCollisionError_ErrorConDetalles(t *testing.T) {
	err := newCalendarPresencialCollisionError([]calendar.PresencialConflict{
		{GroupID: 1, GroupName: "Grupo A", TeamID: 10, TeamName: "Team X", Date: "2026-03-02", PresencialTimeFrom: "09:00", PresencialTimeTo: "10:00"},
		{GroupID: 2, GroupName: "Grupo B", TeamID: 11, TeamName: "Team Y", Date: "2026-03-03", PresencialTimeFrom: "18:00", PresencialTimeTo: "19:00"},
	})
	require.ErrorIs(t, err, ErrCalendarPresencialCollision)
	want := ErrCalendarPresencialCollision.Error() +
		": 2026-03-02 grupo Grupo A (Team X) 09:00-10:00; 2026-03-03 grupo Grupo B (Team Y) 18:00-19:00"
	require.Equal(t, want, err.Error())
}

func TestPresencialCollisionConflicts_ExtraeSoloDeTypedError(t *testing.T) {
	conflicts := []calendar.PresencialConflict{
		{GroupID: 7, GroupName: "G", TeamID: 70, TeamName: "T", Date: "2026-03-02", PresencialTimeFrom: "09:00", PresencialTimeTo: "10:00"},
	}
	require.Equal(t, conflicts, PresencialCollisionConflicts(newCalendarPresencialCollisionError(conflicts)))
	require.Nil(t, PresencialCollisionConflicts(errors.New("otro error")))
	require.Nil(t, PresencialCollisionConflicts(nil))
}

func TestPresencialTimeHHMM(t *testing.T) {
	require.Equal(t, "", presencialTimeHHMM(nil))

	at := time.Date(2026, 3, 2, 12, 5, 0, 0, time.UTC)
	require.Equal(t, "12:05", presencialTimeHHMM(&at))
}
