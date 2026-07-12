package main

import (
	"log"

	_ "IRIS-backend/docs"
	"IRIS-backend/internal/app/server"
)

// @title           LIMS-back API
// @version         2.0
// @description     This is the API server for the LIMS backend.
// @termsOfService  http://swagger.io/terms/
//
// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io
//
// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html
//
// @host      localhost:8443
// @BasePath  /api/v2
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
//
// main はアプリケーションのエントリーポイントです。
// Swagger のドキュメンテーションを生成するために、`swag init -g cmd/server/main.go` を実行してください。
func main() {
	if err := server.Run("config/config.yaml"); err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
}
