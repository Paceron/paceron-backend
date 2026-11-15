package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/utils/gametime"
)

const mergeTestPrimaryGroup int64 = 51

func mergeTestAnchor() time.Time {
	return time.Date(2026, 11, 15, 0, 0, 0, 0, gametime.Location)
}

func int64Pointer(value int64) *int64 { return &value }

func mergeTrainingDay(groupID, teamID, instanceID int64, status string) mergeExistingDay {
	return mergeExistingDay{
		GroupID: groupID, TeamID: teamID, Kind: "training",
		InstanceID: int64Pointer(instanceID), RunnerStatus: status,
	}
}

func TestBuildMergePlanTestingReal(t *testing.T) {
	existing := map[string][]mergeExistingDay{
		// 11-08: día ajeno de group 51, es el corte intencional.
		"2026-11-08": {mergeTrainingDay(51, 39, 801, "")},
		// 11-14: group 51 sin sesión, group 53 ya finished.
		"2026-11-14": {
			mergeTrainingDay(51, 39, 814, ""),
			mergeTrainingDay(53, 29, 914, runnersession.RunnerSessionStatusFinished),
		},
		// 11-15: group 53 sin sesión; group 51 está libre.
		"2026-11-15": {mergeTrainingDay(53, 29, 915, "")},
	}

	plan := buildMergePlan(mergeTestAnchor(), mergeTestPrimaryGroup, existing)
	counts := countMergePlan(plan)

	assert.Equal(t, 7, counts.CreatedDays)
	assert.Equal(t, 2, counts.CompletedExistingDays)
	assert.Equal(t, 9, counts.RunnerSessions)
	assert.Equal(t, 9, counts.Attendances)
	assert.Equal(t, 7, counts.Feedbacks)
	assert.Equal(t, 21, counts.GPSPoints)
	assert.Equal(t, 1, counts.Skipped)

	byDate := map[string]mergeDayPlan{}
	for _, item := range plan.Days {
		byDate[item.Date.Format("2006-01-02")] = item
	}

	assert.False(t, byDate["2026-11-08"].hasActions(), "11-08 es el break y no se toca")
	assert.Contains(t, byDate["2026-11-08"].SkipReason, "break")

	assert.True(t, byDate["2026-11-07"].CreateDay)
	assert.True(t, byDate["2026-11-09"].CreateDay)
	assert.True(t, byDate["2026-11-13"].CreateDay)
	assert.True(t, byDate["2026-11-15"].CreateDay)

	// 11-14 ya está ocupado en el grupo primario: no se crea día, solo se
	// completa la instancia existente sin sesión (group 51).
	day14 := byDate["2026-11-14"]
	assert.True(t, day14.PrimaryOccupied)
	assert.False(t, day14.CreateDay)
	require.Len(t, day14.Completions, 1)
	assert.Equal(t, int64(51), day14.Completions[0].GroupID)
	assert.Equal(t, int64(814), day14.Completions[0].InstanceID)
	assert.Equal(t, "2026-11-14", day14.CompletionDay.Date.Format("2006-01-02"))
	assert.Equal(t, runnersession.RunnerSessionStatusFinished, day14.CompletionDay.RunnerStatus)
	assert.NotNil(t, day14.CompletionDay.End)

	// 11-15 se crea en group 51 y además se completa la instancia de group 53.
	day15 := byDate["2026-11-15"]
	require.Len(t, day15.Completions, 1)
	assert.Equal(t, int64(53), day15.Completions[0].GroupID)
	assert.Equal(t, int64(915), day15.Completions[0].InstanceID)

	// 11-07 va antes del break y debe quedar como día nuevo completo.
	day7 := byDate["2026-11-07"]
	assert.True(t, day7.CreateDay)
	assert.Equal(t, mergeMarkerFor(day7.Date), day7.Day.Marker)
}

func TestBuildMergePlanSinExistentesCreaLaVentanaCompleta(t *testing.T) {
	plan := buildMergePlan(mergeTestAnchor(), mergeTestPrimaryGroup, nil)
	counts := countMergePlan(plan)
	assert.Len(t, plan.Actions, 9)
	assert.Empty(t, plan.Skipped)
	assert.Equal(t, 9, counts.CreatedDays)
	assert.Equal(t, 0, counts.CompletedExistingDays)
	assert.Equal(t, 9, counts.RunnerSessions)
	assert.Equal(t, 9, counts.Attendances)
	assert.Equal(t, 9, counts.Feedbacks)
	assert.Equal(t, 27, counts.GPSPoints)
}

func TestBuildMergePlanIdempotenteConTodoCompleto(t *testing.T) {
	existing := map[string][]mergeExistingDay{}
	for offset := 0; offset < mergeCandidateDays; offset++ {
		date := mergeTestAnchor().AddDate(0, 0, -offset).Format("2006-01-02")
		existing[date] = []mergeExistingDay{
			mergeTrainingDay(mergeTestPrimaryGroup, 39, int64(1000+offset), runnersession.RunnerSessionStatusFinished),
		}
	}
	plan := buildMergePlan(mergeTestAnchor(), mergeTestPrimaryGroup, existing)
	assert.Empty(t, plan.Actions)
	assert.Len(t, plan.Skipped, mergeCandidateDays)
}

func TestBuildMergePlanNoTocaWipNiRest(t *testing.T) {
	existing := map[string][]mergeExistingDay{
		"2026-11-13": {mergeTrainingDay(51, 39, 813, runnersession.RunnerSessionStatusWip)},
		"2026-11-12": []mergeExistingDay{{GroupID: 51, TeamID: 39, Kind: "rest"}},
	}
	plan := buildMergePlan(mergeTestAnchor(), mergeTestPrimaryGroup, existing)
	byDate := map[string]mergeDayPlan{}
	for _, item := range plan.Days {
		byDate[item.Date.Format("2006-01-02")] = item
	}
	assert.False(t, byDate["2026-11-13"].hasActions())
	assert.Contains(t, byDate["2026-11-13"].SkipReason, "wip")
	assert.False(t, byDate["2026-11-12"].hasActions())
	assert.Contains(t, byDate["2026-11-12"].SkipReason, "rest")
}

func TestMergeFixtureDayConsistente(t *testing.T) {
	anchor := mergeTestAnchor()
	date := anchor.AddDate(0, 0, -8)
	day := mergeFixtureDay(anchor, date)
	assert.Equal(t, 8, day.Offset)
	assert.Equal(t, "2026-11-07", day.Date.Format("2006-01-02"))
	assert.Equal(t, 6, day.Start.In(gametime.Location).Hour())
	assert.Equal(t, 45, day.Start.In(gametime.Location).Minute())
	require.NotNil(t, day.End)
	assert.Equal(t, 30*time.Minute, day.End.Sub(day.Start))
	assert.Equal(t, runnersession.RunnerSessionStatusFinished, day.RunnerStatus)
}

func TestMergeMarkerEsDistintoDelMarkerEstricto(t *testing.T) {
	date := mergeTestAnchor()
	mergeMarker := mergeMarkerFor(date)
	strictMarker := markerFor(date)
	assert.Equal(t, "[runner1_merge:2026-11-15]", mergeMarker)
	assert.NotEqual(t, strictMarker, mergeMarker)
	assert.True(t, isFixtureMarker(&strictMarker))
	assert.False(t, isFixtureMarker(&mergeMarker))
}

func TestParseOptionsMergeUsaManifestMergePorDefault(t *testing.T) {
	opts, err := parseForTest(t, []string{"--merge", "--dry-run"})
	require.NoError(t, err)
	assert.True(t, opts.merge)
	assert.Equal(t, defaultMergeManifestPath, opts.manifest)

	strict, err := parseForTest(t, []string{"--dry-run"})
	require.NoError(t, err)
	assert.False(t, strict.merge)
	assert.Equal(t, defaultManifestPath, strict.manifest)

	custom, err := parseForTest(t, []string{"--merge", "--dry-run", "--manifest", "/tmp/custom.json"})
	require.NoError(t, err)
	assert.Equal(t, "/tmp/custom.json", custom.manifest)
}
