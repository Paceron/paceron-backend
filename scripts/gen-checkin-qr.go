// Genera un QR de check-in con el mismo formato que emite el backend.
//
// Útil para probar el escáner del corredor sin depender de la pantalla del
// entrenador: el backend solo emite el QR si el usuario es entrenador del
// equipo, y mientras el binario corra con el commit viejo devuelve la URL de la
// API en vez de la de la pantalla.
//
//   go run scripts/gen-checkin-qr.go <team_id> <session_instance_id> [archivo.png]
//
// Los dos ids son los mismos que viajan en la URL que arma el service:
// `/attendance/register?team_id=%d&session_instance_id=%d`, y el host sale de
// ATTENDANCE_BASE_URL, igual que en attendance_service.go.
package main

import (
	"fmt"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

const qrWebPathFmt = "/attendance/register?team_id=%d&session_instance_id=%d"

// Mismos valores por default que attendance_service.go: el service aplica
// TrimRight, así que una barra final acá daría la misma URL.
func defaultBase() string {
	if v := os.Getenv("ATTENDANCE_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://paceron-frontend.vercel.app"
}

func main() {
	teamID, sessionID, out := 4, int64(503), "checkin-qr.png"

	args := os.Args[1:]
	if len(args) >= 1 {
		if _, err := fmt.Sscan(args[0], &teamID); err != nil {
			fmt.Fprintln(os.Stderr, "team_id debe ser un número:", err)
			os.Exit(1)
		}
	}
	if len(args) >= 2 {
		if _, err := fmt.Sscan(args[1], &sessionID); err != nil {
			fmt.Fprintln(os.Stderr, "session_instance_id debe ser un número:", err)
			os.Exit(1)
		}
	}
	if len(args) >= 3 {
		out = args[2]
	}

	url := fmt.Sprintf("%s%s", defaultBase(), fmt.Sprintf(qrWebPathFmt, teamID, sessionID))
	fmt.Println("URL que va dentro del QR:")
	fmt.Println("  " + url)

	png, err := qrcode.Encode(url, qrcode.Medium, 512)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error generando el QR:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, png, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error escribiendo el archivo:", err)
		os.Exit(1)
	}
	fmt.Printf("QR escrito en %s (%d bytes)\n", out, len(png))
}
