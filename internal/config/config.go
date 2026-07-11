package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

const (
	defaultAppName      = "cloud-deploy-demo-go"
	defaultEnvironment  = "local"
	defaultServerPort   = "8000"
	defaultOTELEndpoint = "http://opentelemetry-collector.observability.svc.cluster.local:4318/v1/traces"
	defaultJDBCURL      = "jdbc:mysql://192.168.50.18:3306/cloud_deploy_demo?createDatabaseIfNotExist=true&useUnicode=true&characterEncoding=utf8&useSSL=false&allowPublicKeyRetrieval=true&serverTimezone=Asia/Shanghai"
)

type Settings struct {
	AppName                    string
	DeploymentEnvironment      string
	ServerPort                 string
	OTELTracesEndpoint         string
	OTELDebugLoggingEnabled    bool
	OTELSDKDisabled            bool
	TracingSamplingProbability float64
	Database                   DatabaseSettings
}

type DatabaseSettings struct {
	Enabled      bool
	DSN          string
	ServerDSN    string
	DatabaseName string
}

func Load() (Settings, error) {
	environment := getenv("DEPLOYMENT_ENVIRONMENT", defaultEnvironment)
	sampling, err := envFloat("MANAGEMENT_TRACING_SAMPLING_PROBABILITY", 1.0)
	if err != nil {
		return Settings{}, err
	}
	if sampling < 0 || sampling > 1 {
		return Settings{}, fmt.Errorf("MANAGEMENT_TRACING_SAMPLING_PROBABILITY must be between 0 and 1")
	}

	database, err := databaseFromEnv()
	if err != nil {
		return Settings{}, err
	}

	return Settings{
		AppName:                    getenv("SPRING_APPLICATION_NAME", defaultAppName),
		DeploymentEnvironment:      environment,
		ServerPort:                 getenv("PORT", getenv("SERVER_PORT", defaultServerPort)),
		OTELTracesEndpoint:         getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", defaultOTELEndpoint),
		OTELDebugLoggingEnabled:    envBool("OTEL_DEBUG_LOGGING_ENABLED", true),
		OTELSDKDisabled:            otelSDKDisabled(environment),
		TracingSamplingProbability: sampling,
		Database:                   database,
	}, nil
}

func databaseFromEnv() (DatabaseSettings, error) {
	if directURL := strings.TrimSpace(os.Getenv("DATABASE_URL")); directURL != "" {
		return mysqlSettingsFromURLOrDSN(directURL)
	}

	if !hasSpringDatasourceEnv() {
		return DatabaseSettings{}, nil
	}

	jdbcURL := getenv("SPRING_DATASOURCE_URL", defaultJDBCURL)
	username := getenv("SPRING_DATASOURCE_USERNAME", "root")
	password := os.Getenv("SPRING_DATASOURCE_PASSWORD")
	if !strings.HasPrefix(jdbcURL, "jdbc:mysql://") {
		return DatabaseSettings{}, errors.New("SPRING_DATASOURCE_URL must start with jdbc:mysql://")
	}

	return mysqlSettingsFromJDBC(jdbcURL, username, password)
}

func mysqlSettingsFromJDBC(jdbcURL, username, password string) (DatabaseSettings, error) {
	parsed, err := url.Parse(strings.TrimPrefix(jdbcURL, "jdbc:"))
	if err != nil {
		return DatabaseSettings{}, fmt.Errorf("parse SPRING_DATASOURCE_URL: %w", err)
	}
	if parsed.Host == "" {
		return DatabaseSettings{}, errors.New("SPRING_DATASOURCE_URL host is required")
	}

	databaseName := strings.TrimPrefix(parsed.Path, "/")
	if databaseName == "" {
		return DatabaseSettings{}, errors.New("SPRING_DATASOURCE_URL database name is required")
	}

	query := parsed.Query()
	charset := firstNonEmpty(query.Get("charset"), query.Get("characterEncoding"), "utf8mb4")
	params := url.Values{}
	params.Set("charset", charset)
	params.Set("parseTime", "true")
	params.Set("loc", "Local")

	address := ensurePort(parsed.Host, "3306")
	dsn := mysqlDSN(username, password, address, databaseName, params)
	serverDSN := mysqlDSN(username, password, address, "", params)

	return DatabaseSettings{
		Enabled:      true,
		DSN:          dsn,
		ServerDSN:    serverDSN,
		DatabaseName: databaseName,
	}, nil
}

func mysqlSettingsFromURLOrDSN(raw string) (DatabaseSettings, error) {
	if isMySQLURL(raw) {
		normalizedURL := strings.Replace(raw, "mysql+pymysql://", "mysql://", 1)
		parsed, err := url.Parse(normalizedURL)
		if err != nil {
			return DatabaseSettings{}, fmt.Errorf("parse DATABASE_URL: %w", err)
		}
		username := parsed.User.Username()
		password, _ := parsed.User.Password()
		databaseName := strings.TrimPrefix(parsed.Path, "/")
		if parsed.Host == "" || databaseName == "" {
			return DatabaseSettings{}, errors.New("DATABASE_URL must include host and database name")
		}

		params := parsed.Query()
		if params.Get("charset") == "" {
			params.Set("charset", "utf8mb4")
		}
		params.Set("parseTime", "true")
		if params.Get("loc") == "" {
			params.Set("loc", "Local")
		}

		address := ensurePort(parsed.Host, "3306")
		return DatabaseSettings{
			Enabled:      true,
			DSN:          mysqlDSN(username, password, address, databaseName, params),
			ServerDSN:    mysqlDSN(username, password, address, "", params),
			DatabaseName: databaseName,
		}, nil
	}

	return DatabaseSettings{Enabled: true, DSN: raw}, nil
}

func isMySQLURL(raw string) bool {
	return strings.HasPrefix(raw, "mysql://") || strings.HasPrefix(raw, "mysql+pymysql://")
}

func mysqlDSN(username, password, address, database string, params url.Values) string {
	config := mysql.Config{
		User:                 username,
		Passwd:               password,
		Net:                  "tcp",
		Addr:                 address,
		DBName:               database,
		AllowNativePasswords: true,
		Params:               map[string]string{},
	}
	for key, values := range params {
		if len(values) > 0 {
			config.Params[key] = values[len(values)-1]
		}
	}
	return config.FormatDSN()
}

func hasSpringDatasourceEnv() bool {
	return os.Getenv("SPRING_DATASOURCE_URL") != "" ||
		os.Getenv("SPRING_DATASOURCE_USERNAME") != "" ||
		os.Getenv("SPRING_DATASOURCE_PASSWORD") != ""
}

func otelSDKDisabled(environment string) bool {
	if value, ok := os.LookupEnv("OTEL_SDK_DISABLED"); ok {
		return parseBool(value)
	}
	return environment == defaultEnvironment && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == ""
}

func getenv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	return parseBool(value)
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func envFloat(name string, fallback float64) (float64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", name, err)
	}
	return parsed, nil
}

func ensurePort(host, fallbackPort string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(host, fallbackPort)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
