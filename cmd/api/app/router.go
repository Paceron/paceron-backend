package app

import (
	"fmt"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"

	"github.com/gin-gonic/gin"
)

const banner = `
 ____   _    ____ _____ ____   ___  _   _ 
|  _ \ / \  / ___| ____|  _ \ / _ \| \ | |
| |_) / _ \| |   |  _| | |_) | | | |  \| |
|  __/ ___ \ |___| |___|  _ <| |_| | |\  |
|_| /_/   \_\____|_____|_| \_\\___/|_| \_|
`

// bannerLocal se imprime SOLO bajo --stage=local. El log "stage resolved" dice
// a que ambiente se resolvio, pero pasa desapercibido en un scroll de arranque;
// esto es lo que uno ve. La ballena es el logo de Docker a proposito: el mensaje
// es "esto corre contra el stack de Docker", no "estoy en develop".
//
// Importa que la info sea accionable y no decorativa: los tres comandos del
// quickstart (local-logs, local-reset, local-restore) son lo que uno necesita
// cuando se acuerda de que esta mal, y el reminder del flag evita el error
// clasico — olvidarse del flag y estar pegando contra Supabase sin saberlo.
const bannerLocal = `
                       ##
                     ##  ##
     ##   ##    ##   ##  ##  ##
   ##  ## ##  ## ##  ##  ##  ## ##
  ##  ##  ##  ##  ##  ##  ##  ## ##
  ## ##    ########  ##  ##  ## ##
   ####   ##      ## ##  ##  ## ##
     ##  ##       ## ##  ##  ##
      ##  ##       ##########
       ####       ##  ##  ##
         ##        ####  ####

  ENTORNO LOCAL DOCKERIZADO

  base     postgresql://postgres:postgres@localhost:5432/paceron_local
  storage  http://localhost:9000     consola http://localhost:9001
  bucket   paceron-media (lectura publica)

  make local-logs      ver logs        make local-reset    borrar todo
  make local-restore   recargar la base desde Supabase

  Sin --stage=local esto no aparece: el flag es lo unico que activa el stack.
  No tenes flag? estas pegando contra Supabase. Ver docs/ENTORNO_LOCAL.md
`

func StartApp() {
	fmt.Print(banner)
	if config.IsLocalStage() {
		fmt.Print(bannerLocal)
	}
	customlogger.CustomConfig(customlogger.DebugLevel, true, true, true)
	customlogger.SetShowURL(true)

	// Loguear el stage resuelto no es un detalle: los tres se ven igual desde
	// afuera (misma app, mismos endpoints) y confundirse produce síntomas que
	// parecen bugs. El caso típico es haberse olvidado de --stage=local y estar
	// pegados contra la base de Supabase cloud sin saberlo.
	stage := "testing"
	switch {
	case config.IsLocalStage():
		stage = "LOCAL"
	case config.IsProductionStage():
		stage = "PRODUCTION"
	}
	customlogger.Info(nil, "stage resolved", customlogger.Tag("stage", stage))

	router := gin.New()
	router.Use(gin.Recovery())
	app := NewApplication()
	mapUrls(router, app)

	for _, route := range router.Routes() {
		customlogger.Info(nil, "endpoint registered",
			customlogger.Tag("method", route.Method),
			customlogger.Tag("path", route.Path),
			customlogger.Tag("handler", route.Handler))
	}

	if err := router.Run(":8080"); err != nil {
		customlogger.Error(nil, "error when trying to start the application", err)
		panic(err)
	}
}
