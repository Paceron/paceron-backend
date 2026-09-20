package instance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
)

func TestNewSessionResponse_MapsExercisesByID(t *testing.T) {
	sess := dbs.SessionInstance{ID: 1, Name: "Fartlek 5K"}
	warmup := dbs.ExerciseInstance{ID: 10, Name: "Trote", Kind: "jogging"}
	main := dbs.ExerciseInstance{ID: 20, Name: "Serie", Kind: "running"}
	links := []dbs.SessionExerciseInstance{
		{ID: 100, SessionInstanceID: 1, ExerciseInstanceID: 20, Role: "main", RepeatCount: 3, RestMinutes: 2},
		{ID: 101, SessionInstanceID: 1, ExerciseInstanceID: 10, Role: "warmup"},
	}
	exercises := []dbs.ExerciseInstance{warmup, main}

	resp, err := NewSessionResponse(sess, links, exercises)

	require.NoError(t, err)
	assert.Equal(t, sess.ID, resp.ID)
	require.Len(t, resp.Exercises, 2)
	assert.Equal(t, "main", resp.Exercises[0].Role)
	assert.Equal(t, "Serie", resp.Exercises[0].Name)
	assert.Equal(t, "warmup", resp.Exercises[1].Role)
	assert.Equal(t, 0, resp.Exercises[1].RepeatCount)
}

func TestNewSessionResponse_MissingExercise_ReturnsError(t *testing.T) {
	sess := dbs.SessionInstance{ID: 1, Name: "Fartlek 5K"}
	link := dbs.SessionExerciseInstance{ID: 100, SessionInstanceID: 1, ExerciseInstanceID: 20, Role: "main"}

	resp, err := NewSessionResponse(sess, []dbs.SessionExerciseInstance{link}, nil)

	assert.Error(t, err)
	assert.Empty(t, resp)
}

func TestNewSessionResponse_NoLinks_ReturnsEmptyExercises(t *testing.T) {
	sess := dbs.SessionInstance{ID: 1, Name: "Descarga"}

	resp, err := NewSessionResponse(sess, nil, nil)

	require.NoError(t, err)
	assert.NotNil(t, resp.Exercises)
	assert.Empty(t, resp.Exercises)
}
