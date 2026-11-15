package main

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/utils/gametime"
)

func parseForTest(t *testing.T, args []string) (options, error) {
	t.Helper()
	fs := flag.NewFlagSet("seed_runner1_test", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return parseOptionsArgs(fs, args)
}

func TestParseOptionsDefaultsADryRunLocal(t *testing.T) {
	opts, err := parseForTest(t, nil)
	require.NoError(t, err)
	assert.True(t, opts.dryRun)
	assert.False(t, opts.apply)
	assert.False(t, opts.cleanup)
	assert.Equal(t, "local", opts.target)
	assert.Equal(t, defaultEmail, opts.email)
	assert.Equal(t, defaultManifestPath, opts.manifest)
}

func TestParseOptionsModosExcluyentes(t *testing.T) {
	_, err := parseForTest(t, []string{"--apply", "--cleanup"})
	assert.Error(t, err)

	_, err = parseForTest(t, []string{"--dry-run", "--apply"})
	assert.Error(t, err)

	_, err = parseForTest(t, []string{"--target=production"})
	assert.Error(t, err)

	_, err = parseForTest(t, []string{"--email", "  "})
	assert.Error(t, err)
}

func TestParseDSN(t *testing.T) {
	db, err := parseDSN("postgresql://user:pass@localhost:5433/paceron_test")
	require.NoError(t, err)
	assert.Equal(t, "localhost", db.Host)
	assert.Equal(t, "5433", db.Port)
	assert.Equal(t, "user", db.Username)
	assert.Equal(t, "pass", db.Password)
	assert.Equal(t, "paceron_test", db.Name)

	db, err = parseDSN("postgres://user:pass@localhost/db")
	require.NoError(t, err)
	assert.Equal(t, "5432", db.Port)

	_, err = parseDSN("not a url")
	assert.Error(t, err)
}

func TestIsLocalHost(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		assert.True(t, isLocalHost(host), host)
	}
	for _, host := range []string{"db.supabase.co", "10.0.0.5", ""} {
		assert.False(t, isLocalHost(host), host)
	}
}

func TestResolveAnchor(t *testing.T) {
	orig := gametime.Now
	t.Cleanup(func() { gametime.Now = orig })
	gametime.Now = func() time.Time {
		return time.Date(2026, 11, 15, 12, 0, 0, 0, gametime.Location)
	}

	anchor, err := resolveAnchor("")
	require.NoError(t, err)
	assert.Equal(t, "2026-11-15", anchor.Format("2006-01-02"))

	anchor, err = resolveAnchor("2026-11-15")
	require.NoError(t, err)
	assert.Equal(t, "2026-11-15", anchor.Format("2006-01-02"))

	_, err = resolveAnchor("2026-11-14")
	assert.Error(t, err)

	_, err = resolveAnchor("15/11/2026")
	assert.Error(t, err)
}

func TestUTCDateInCordobaSigueLaConvencionDelHistorico(t *testing.T) {
	// 2026-11-04 02:00 UTC = 2026-11-03 23:00 en Córdoba: el día lógico es el
	// de UTC, igual que streak_service.utcDateToDay.
	value := time.Date(2026, 11, 4, 2, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-11-04", utcDateInCordoba(value).Format("2006-01-02"))
}

func TestResolveTargetRechazaProduccion(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"seed_runner1", "--stage=production"}

	_, err := resolveTarget(options{target: "local"})
	assert.Error(t, err)
}

func TestResolveTestingTargetRequiereGuardas(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"seed_runner1"}

	t.Setenv("SEEDING_ALLOW_REMOTE", "")
	_, err := resolveTarget(options{target: "testing"})
	assert.Error(t, err)

	t.Setenv("SEEDING_ALLOW_REMOTE", "true")
	t.Setenv("SUPABASE_TESTING_DATABASE_URL", "")
	_, err = resolveTarget(options{target: "testing"})
	assert.Error(t, err)

	t.Setenv("SUPABASE_TESTING_DATABASE_URL", "postgres://u:p@testing.example.com:5432/db")
	t.Setenv("SUPABASE_PRODUCTION_DATABASE_URL", "postgres://u:p@prod.example.com:5432/db")
	tgt, err := resolveTarget(options{target: "testing"})
	require.NoError(t, err)
	assert.True(t, tgt.remote)
	assert.Equal(t, "testing.example.com", tgt.db.Host)

	t.Setenv("SUPABASE_PRODUCTION_DATABASE_URL", "postgres://u:p@testing.example.com:5432/db")
	_, err = resolveTarget(options{target: "testing"})
	assert.Error(t, err)
}

func TestResolveLocalTargetConDBOverride(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"seed_runner1"}

	tgt, err := resolveTarget(options{
		target:     "local",
		dbOverride: "postgres://u:p@localhost:5433/paceron_test",
	})
	require.NoError(t, err)
	assert.False(t, tgt.remote)
	assert.Equal(t, "localhost", tgt.db.Host)
	assert.Equal(t, "5433", tgt.db.Port)
}
