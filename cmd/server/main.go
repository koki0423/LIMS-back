package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	_ "IRIS-backend/docs"
	"IRIS-backend/internal/app/server"
)

const defaultConfigPath = "config/config.yaml"

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
	configPath, err := parseConfigPath(os.Args[1:])
	if err != nil {
		log.Fatalf("[FATAL] failed to parse arguments: %v", err)
	}

	if err := server.Run(configPath); err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
}

func parseConfigPath(args []string) (string, error) {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	configPath := fs.String("config", defaultConfigPath, "path to config file")

	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if fs.NArg() > 0 {
		return "", fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	return *configPath, nil
}
