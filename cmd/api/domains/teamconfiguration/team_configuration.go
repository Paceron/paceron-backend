package teamconfiguration

// TeamConfiguration define los topes de creación/edición de equipos que un
// entrenador puede usar según su tier. Es a la vez el valor del hashmap por
// tier y el DTO de respuesta del endpoint.
type TeamConfiguration struct {
	MaxMembers int     `json:"max_members"`
	MinimumFee float64 `json:"minimum_fee"`
}

const (
	// DefaultMaxMembers es la cantidad máxima de integrantes si el tier del
	// entrenador no está en el mapa (o no hay tier resuelto).
	DefaultMaxMembers = 10
	// DefaultMinimumFee es el valor mínimo de membership_fee en el mismo caso.
	DefaultMinimumFee = 20000
)

// teamTierConfigurations es el hashmap tier → configuración de equipo. Si el
// tier no está acá, ForTier devuelve el default. Los valores son placeholder
// acordados con producto y se ajustan solo en este archivo.
var teamTierConfigurations = map[string]TeamConfiguration{
	"base":    {MaxMembers: 10, MinimumFee: DefaultMinimumFee},
	"medium":  {MaxMembers: 25, MinimumFee: DefaultMinimumFee},
	"premium": {MaxMembers: 50, MinimumFee: DefaultMinimumFee},
}

// ForTier devuelve la configuración del tier dado; si el tier no está en el
// mapa devuelve el default (10 integrantes / fee mínimo 20000).
func ForTier(tierName string) TeamConfiguration {
	if cfg, ok := teamTierConfigurations[tierName]; ok {
		return cfg
	}
	return TeamConfiguration{MaxMembers: DefaultMaxMembers, MinimumFee: DefaultMinimumFee}
}