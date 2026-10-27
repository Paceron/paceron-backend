package dashboard

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// El bloque de unidades es único y va completo en las seis respuestas: si una
// respuesta lo armo por su cuenta, tarde o temprano queda una desactualizada.
func TestUnitsBlock(t *testing.T) {
	raw, err := json.Marshal(UnitsBlock)

	assert.NoError(t, err)
	assert.JSONEq(t, `{"speed":"km/h","time":"ms","distance":"m","delta":"%","compliance":"%","best_time":"ms","percentage_point":"pp"}`, string(raw))
}

func TestBandCountsVieneConLasCuatroClaves(t *testing.T) {
	raw, err := json.Marshal(BandCounts{})

	assert.NoError(t, err)
	assert.JSONEq(t, `{"met":0,"near":0,"missed":0,"unknown":0}`, string(raw))
}
