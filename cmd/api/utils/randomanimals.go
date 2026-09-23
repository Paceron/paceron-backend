package utils

import (
	"math/rand"
)

// RandomJungleAnimal devuelve el nombre de un animal de la selva aleatorio.
// Los animales están seleccionados por sus habilidades naturales que evocan
// los deportes de running (velocidad, trail, resistencia) o marítimos (natación, buceo).
func RandomJungleAnimal() string {
	animals := []string{
		// 🏃‍♂️ Relacionados con RUNNING (Velocidad, Trail Running, Resistencia)
		"Jaguares",         // Sprint: Máxima explosividad y velocidad punta.
		"Ocelotes",         // Trail Running: Agilidad extrema entre obstáculos.
		"Agutíes",          // Velocidad: Corredor rapidísimo en distancias cortas.
		"Tapirres",         // Ultra-running: Gran resistencia para largas caminatas/carreras.
		"Venados",          // Campo a través: Saltos y carreras por terreno irregular.
		"Pacas",            // Sprint: Velocidad y reflejos rápidos.
		"Monos Aulladores", // Trail Running: Movilidad y resistencia en el "trail" arbóreo.
		"Monos Ardilla",    // Parkour/Agilidad: Saltos y movimientos acrobáticos.
		"Pecaríes",         // Cross-country: Resistencia y capacidad para recorrer largas distancias.

		// 🏊‍♂️ Relacionados con DEPORTES MARÍTIMOS / ACUÁTICOS (Natación, Buceo, Aguas abiertas)
		"Nutrias",    // Natación: El "lobo de río", excelente nadador de aguas abiertas.
		"Caimánes",   // Buceo: Natación subacuática y apnea.
		"Capibaras",  // Natación: Nadador de resistencia y semi-acuático.
		"Delfínes",   // Natación sincronizada: Elegancia y técnica en el agua.
		"Peces",      // Acuático: Famoso por "correr" sobre el agua.
		"Pirañas",    // Aguas bravas: Natación en ríos de corriente fuerte.
		"Manatíes",   // Natación suave: Flotabilidad y aguas tranquilas.
		"Tortugas",   // Natación de fondo: Resistencia y nado de larga distancia.
		"Cangrejos",  // Deportes de playa/marea: Vida en la interfaz selva-mar.
		"Pirarucúes", // Buceo en profundidad: Gigante de las aguas profundas.
		"Ranas",      // Saltos y natación: Movilidad acuática y saltos desde altura.
		"Caballos",
		"Tigres",
		"Leones",
		"Delfines",
		"Zuricatas",
		"Cabras",
		"Cobras",
		"Cebras",
		"Culebras",
		"Pumas",
		"Gacelas",
		"Elefantes",
		"Jirafas",
		"Rinocerontes",
		"Hipopótamos",
		"Orangutanes",
		"Gorilas",
		"Chimpancés",
		"Rinocerontes",
		"Hipopótamos",
		"Orangutanes",
		"Gorilas",
	}
	// En Go 1.20+, el generador aleatorio ya viene sembrado automáticamente.
	// Si usas una versión anterior, descomenta la siguiente línea:
	// rand.Seed(time.Now().UnixNano())

	randomIndex := rand.Intn(len(animals))
	return animals[randomIndex]
}
