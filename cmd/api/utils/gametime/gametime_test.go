package gametime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Los tests del helper fijan Now con time.Date (design D1: determinismo, nada
// de horas reales del runner en decisiones de día).

func TestGametime_LocationEsCordoba(t *testing.T) {
	assert.Equal(t, "America/Argentina/Cordoba", Location.String())
}

func TestGametime_DateOnlyEnCordoba(t *testing.T) {
	// 2026-10-14 02:00 UTC = 2026-10-13 23:00 en Cordoba: el día NO se pasa
	// por el offset, o sea queda 2026-10-13.
	atTwoUTC := time.Date(2026, 10, 14, 2, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-10-13", DateOnly(atTwoUTC).Format("2006-01-02"))
	// 2026-10-13 21:00 UTC = 2026-10-13 18:00 Cordoba: mismo día local.
	atNineUTC := time.Date(2026, 10, 13, 21, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-10-13", DateOnly(atNineUTC).Format("2006-01-02"))
	med := DateOnly(atTwoUTC)
	assert.Equal(t, Location, med.Location())
	assert.Equal(t, "-03", med.Format("-07"))

	// La medianoche local es 03:00 UTC: al 04:00 UTC ya corrió el día en
	// Cordoba, al 02:00 UTC todavía no.
	afterMidnightUTC := time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-10-14", DateOnly(afterMidnightUTC).Format("2006-01-02"))
}

func TestGametime_TodayYesterdayConNowOverride(t *testing.T) {
	fixedNow := time.Date(2026, 10, 13, 23, 59, 59, 0, Location)
	orig := Now
	t.Cleanup(func() { Now = orig })
	Now = func() time.Time { return fixedNow }

	today := Today()
	require.Equal(t, Location, today.Location())
	assert.Equal(t, "2026-10-13", today.Format("2006-01-02"))
	assert.Equal(t, time.Date(2026, 10, 13, 0, 0, 0, 0, Location), today)

	yesterday := Yesterday()
	assert.Equal(t, "2026-10-12", yesterday.Format("2006-01-02"))
	assert.Equal(t, time.Date(2026, 10, 12, 0, 0, 0, 0, Location), yesterday)
}

// Un override con hora avanza por el In(Location): el "hoy" sale del wall clock
// fijo pero siempre colapsado a medianoche local.
func TestGametime_TodayColapsaHorasFijadas(t *testing.T) {
	orig := Now
	t.Cleanup(func() { Now = orig })
	Now = func() time.Time {
		return time.Date(2026, 3, 1, 16, 30, 0, 0, time.UTC) // 13:30 Cordoba
	}

	today := Today()
	assert.Equal(t, "2026-03-01", today.Format("2006-01-02"))
	assert.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, Location), today)
}
