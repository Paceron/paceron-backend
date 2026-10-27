package dashboard

// Units es el bloque de unidades que viaja en las seis respuestas de métricas:
// el front rotula los ejes con esto y el CSV escribe la unidad en la cabecera de
// cada columna, así que no puede pedirla de otro endpoint.
type Units struct {
	Speed           string `json:"speed"`
	Time            string `json:"time"`
	Distance        string `json:"distance"`
	Delta           string `json:"delta"`
	Compliance      string `json:"compliance"`
	BestTime        string `json:"best_time"`
	PercentagePoint string `json:"percentage_point"`
}

// UnitsBlock es el valor único de ese bloque. Las siete claves son fijas y
// duplicar el mapa en cada response es la forma de que una quede sin
// actualizar, así que se inyecta el mismo valor en todas.
var UnitsBlock = Units{
	Speed:           "km/h",
	Time:            "ms",
	Distance:        "m",
	Delta:           "%",
	Compliance:      "%",
	BestTime:        "ms",
	PercentagePoint: "pp",
}
