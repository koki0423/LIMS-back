package db

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

const (
	driverName     = "mysql"
	configFilePath = "config/config.yaml"
)

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type Certs struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

type YahooConfig struct {
	AppID string `yaml:"app_id"`
}

type Config struct {
	Version     string         `yaml:"version"`
	Mode        string         `yaml:"mode"`
	DB          DatabaseConfig `yaml:"database"`
	Certificate Certs          `yaml:"certificate"`
	Yahoo       YahooConfig    `yaml:"yahoo"`
}

// LoadConfig はYAMLファイルを読み込みますが、ファイルが存在しない場合は環境変数を使用します
func LoadConfig(path string) (*Config, error) {
	// ファイルが存在するかチェック
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("[INFO] config.yaml not found. Falling back to environment variables.")
		return loadFromEnv(), nil
	}

	// yamlファイルから読み込む
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込み失敗: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(buf, &cfg); err != nil {
		return nil, fmt.Errorf("設定ファイルのパース失敗: %w", err)
	}
	return &cfg, nil
}

// loadFromEnv は環境変数からConfig構造体を組み立てます
func loadFromEnv() *Config {
	return &Config{
		Version: getEnv("APP_VERSION", "1.0"),
		Mode:    getEnv("APP_MODE", "release"),
		DB: DatabaseConfig{
			Host:     getEnv("DB_HOST", "mysql"),
			Port:     getEnvAsInt("DB_PORT", 3306),
			Username: getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", "test1234"),
			DBName:   getEnv("DB_NAME", "lims_v1"),
		},
		Certificate: Certs{
			Cert: getEnv("CERT_FILE", ""),
			Key:  getEnv("KEY_FILE", ""),
		},
		Yahoo: YahooConfig{
			AppID: getEnv("YAHOO_APP_ID", ""),
		},
	}
}

// --- ヘルパー関数 ---

// getEnv は環境変数を取得し、空の場合はフォールバック値を返します
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvAsInt は環境変数を整数として取得し、変換できない場合や空の場合はフォールバック値を返します
func getEnvAsInt(key string, fallback int) int {
	strValue := getEnv(key, "")
	if value, err := strconv.Atoi(strValue); err == nil {
		return value
	}
	return fallback
}

func Connect(c DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&tls=false&timeout=3s&readTimeout=5s&writeTimeout=5s&loc=UTC",
		c.Username, c.Password, c.Host, c.Port, c.DBName)

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("接続準備に失敗: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("DB接続に失敗: %w", err)
	}

	// 接続プール（合算がMySQLの max_connections を超えないよう配分する）
	db.SetMaxOpenConns(80)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	return db, nil
}