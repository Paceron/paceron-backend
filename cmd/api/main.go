package main

import "simple-arq-golang/cmd/api/app"

// @title           Paceron Backend API
// @version         1.0
// @description     API para el registro y gestión de usuarios de Paceron
// @termsOfService  http://swagger.io/terms/
// @contact.name    API Support
// @contact.email   dev@paceron.com
// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT
// @host            localhost:8080
// @BasePath        /
func main() {
	app.StartApp()
}
