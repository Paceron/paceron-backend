package main

import (
	"flag"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/utils/gametime"
)

func newTestFlagSet() *flag.FlagSet {
	return flag.NewFlagSet("delete_training_data_test", flag.ContinueOnError)
}

func TestParseOptionsArgs_Valido(t *testing.T) {
	opts, err := parseOptionsArgs(newTestFlagSet(), []string{"--team=29", "--from=2026-11-15"})
	require.NoError(t, err)

	assert.Equal(t, int64(29), opts.team)
	assert.Equal(t, "2026-11-15", opts.from)
	assert.Empty(t, opts.to)
	assert.False(t, opts.apply)
	assert.Equal(t, "local", opts.target)
}

func TestParseOptionsArgs_TeamYFromObligatorios(t *testing.T) {
	_, err := parseOptionsArgs(newTestFlagSet(), []string{"--from=2026-11-15"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--team")

	_, err = parseOptionsArgs(newTestFlagSet(), []string{"--team=29"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--from")
}

func TestParseOptionsArgs_TargetInvalido(t *testing.T) {
	_, err := parseOptionsArgs(newTestFlagSet(), []string{"--team=29", "--from=2026-11-15", "--target=production"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--target")
}

func TestParseOptionsArgs_YesSoloConTesting(t *testing.T) {
	_, err := parseOptionsArgs(newTestFlagSet(), []string{"--team=29", "--from=2026-11-15", "--yes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}

func TestNewScope_Fechas(t *testing.T) {
	sc, err := newScope(options{team: 29, from: "2026-11-15"})
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 15, 0, 0, 0, 0, gametime.Location), sc.from)
	assert.Nil(t, sc.to)
	assert.Equal(t, []string{"rest", "other", "training", "cancelled"}, sc.kinds)

	sc, err = newScope(options{team: 29, from: "2026-11-15", to: "2026-11-20"})
	require.NoError(t, err)
	require.NotNil(t, sc.to)
	assert.Equal(t, time.Date(2026, 11, 20, 0, 0, 0, 0, gametime.Location), *sc.to)
}

func TestNewScope_DesdeInvalidoYToAnterior(t *testing.T) {
	_, err := newScope(options{team: 29, from: "15/11/2026"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "YYYY-MM-DD")

	_, err = newScope(options{team: 29, from: "2026-11-15", to: "2026-11-14"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anterior")
}

func TestParseKinds(t *testing.T) {
	kinds, err := parseKinds("training, rest,training")
	require.NoError(t, err)
	assert.Equal(t, []string{"training", "rest"}, kinds)

	_, err = parseKinds("training,nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")

	_, err = parseKinds(" , ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vacío")
}

func TestParseIDs(t *testing.T) {
	ids, err := parseIDs("39, 40")
	require.NoError(t, err)
	assert.Equal(t, []int64{39, 40}, ids)

	_, err = parseIDs("39,x")
	require.Error(t, err)
}

func TestUntilExclusive(t *testing.T) {
	assert.Nil(t, untilExclusive(nil))

	to := time.Date(2026, 11, 20, 0, 0, 0, 0, gametime.Location)
	got := untilExclusive(&to)
	require.NotNil(t, got)
	assert.Equal(t, time.Date(2026, 11, 21, 0, 0, 0, 0, gametime.Location), *got)
}

func TestChunkIDs(t *testing.T) {
	assert.Nil(t, chunkIDs(nil))

	ids := make([]int64, chunkSize+3)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	chunks := chunkIDs(ids)
	require.Len(t, chunks, 2)
	assert.Len(t, chunks[0], chunkSize)
	assert.Len(t, chunks[1], 3)
}

func TestFormatKindCounts(t *testing.T) {
	assert.Equal(t, "", formatKindCounts(nil))
	assert.Equal(t, "(other=1, training=2)", formatKindCounts(map[string]int{"training": 2, "other": 1}))
}
