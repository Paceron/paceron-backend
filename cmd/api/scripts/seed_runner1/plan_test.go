package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/utils/gametime"
)

func fixedAnchor() time.Time {
	return time.Date(2026, 11, 15, 0, 0, 0, 0, gametime.Location)
}

func TestExpectedAchievementSetsCubrenElCatalogo(t *testing.T) {
	unlocked, locked, err := expectedAchievementSets()
	require.NoError(t, err)
	assert.Len(t, unlocked, 13)
	assert.Len(t, locked, 8)
	assert.Equal(t, expectedUnlockedAchievements, unlocked)
	assert.Equal(t, expectedLockedAchievements, locked)
}

func TestBuildFixturePlanTimelineYAgregados(t *testing.T) {
	anchor := fixedAnchor()
	plan := buildFixturePlan(anchor)

	require.Len(t, plan.Days, 30)
	counts := map[string]int{}
	var finished, early, feedback, gps, attendance int
	var distance, maxSingle float64
	for _, day := range plan.Days {
		counts[day.DayStatus]++
		if day.RunnerStatus == runnersession.RunnerSessionStatusFinished {
			finished++
			if day.Start.In(gametime.Location).Hour() < 8 {
				early++
			}
		}
		if day.HasFeedback {
			feedback++
			distance += day.DistanceMeters
			if day.DistanceMeters > maxSingle {
				maxSingle = day.DistanceMeters
			}
		}
		if day.HasGPS {
			gps++
		}
		if day.HasAttendance {
			attendance++
		}
	}

	assert.Equal(t, 28, finished)
	assert.Equal(t, 5, early)
	assert.Equal(t, 28, counts["completed"])
	assert.Equal(t, 2, counts["failed"])
	assert.Equal(t, 12, feedback)
	assert.Equal(t, 10, gps)
	assert.Equal(t, 10, attendance)
	assert.Equal(t, float64(65000), distance)
	assert.Equal(t, float64(21000), maxSingle)

	assert.Equal(t, 28, plan.ExpectedFinishedSessions)
	assert.Equal(t, 5, plan.ExpectedEarlySessions)
	assert.Equal(t, float64(65000), plan.ExpectedDistanceMeters)
	assert.Equal(t, float64(21000), plan.ExpectedMaxSingleMeters)
	assert.Equal(t, 10, plan.ExpectedGPSDays)
	assert.Equal(t, 10, plan.ExpectedAttendanceDays)
	assert.Equal(t, 10, plan.ExpectedStreakCurrent)
	assert.Equal(t, 14, plan.ExpectedStreakBest)
}

func TestBuildFixturePlanSeparadoresYVentana(t *testing.T) {
	anchor := fixedAnchor()
	plan := buildFixturePlan(anchor)

	yesterday := anchor.AddDate(0, 0, -1)
	assert.Equal(t, yesterday, plan.Days[0].Date)
	assert.Equal(t, anchor.AddDate(0, 0, -30), plan.Days[29].Date)

	for _, offset := range []int{11, 26} {
		day := plan.Days[offset-1]
		assert.Equal(t, "failed", day.DayStatus, "offset %d", offset)
		assert.Equal(t, runnersession.RunnerSessionStatusWip, day.RunnerStatus, "offset %d", offset)
		assert.Nil(t, day.End, "offset %d", offset)
	}

	for _, offset := range []int{1, 5, 6, 10, 12, 25, 27, 30} {
		day := plan.Days[offset-1]
		assert.Equal(t, "completed", day.DayStatus, "offset %d", offset)
		assert.Equal(t, runnersession.RunnerSessionStatusFinished, day.RunnerStatus, "offset %d", offset)
		assert.NotNil(t, day.End, "offset %d", offset)
	}
}

func TestBuildFixturePlanEarlyExactamenteCinco(t *testing.T) {
	plan := buildFixturePlan(fixedAnchor())
	for _, day := range plan.Days {
		if day.RunnerStatus != runnersession.RunnerSessionStatusFinished {
			continue
		}
		isEarly := day.Start.In(gametime.Location).Hour() < 8
		if day.Offset <= 5 {
			assert.True(t, isEarly, "offset %d debería ser early", day.Offset)
		} else {
			assert.False(t, isEarly, "offset %d no debería ser early", day.Offset)
		}
	}
}

func TestBuildFixturePlanFeedback(t *testing.T) {
	plan := buildFixturePlan(fixedAnchor())
	feedbackOffsets := make(map[int]fixtureDay)
	for _, day := range plan.Days {
		if day.HasFeedback {
			feedbackOffsets[day.Offset] = day
		}
	}
	for offset := 1; offset <= 10; offset++ {
		day, ok := feedbackOffsets[offset]
		require.True(t, ok, "offset %d debería tener feedback", offset)
		assert.Equal(t, float64(4000), day.DistanceMeters)
		assert.True(t, day.HasGPS)
	}
	assert.Equal(t, float64(4000), feedbackOffsets[24].DistanceMeters)
	assert.False(t, feedbackOffsets[24].HasGPS)
	assert.Equal(t, float64(21000), feedbackOffsets[25].DistanceMeters)
	assert.False(t, feedbackOffsets[25].HasGPS)
}

func TestMarkerForEsEstableYPrefijado(t *testing.T) {
	date := time.Date(2026, 11, 15, 13, 45, 0, 0, gametime.Location)
	assert.Equal(t, "[runner1_data:2026-11-15]", markerFor(date))
	assert.Equal(t, markerFor(date), markerFor(time.Date(2026, 11, 15, 0, 0, 0, 0, gametime.Location)))
	marker := markerFor(date)
	assert.True(t, isFixtureMarker(&marker))
	other := "[seed_full:x]"
	assert.False(t, isFixtureMarker(&other))
	assert.False(t, isFixtureMarker(nil))
}

func TestStampSourceNameLlevaMarker(t *testing.T) {
	name := stampSourceName(1, "Competencia de prueba · 5K")
	assert.True(t, isFixtureStampName(&name))
	other := "Competencia real"
	assert.False(t, isFixtureStampName(&other))
	assert.False(t, isFixtureStampName(nil))
}
