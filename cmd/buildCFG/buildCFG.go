package buildCFG

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/config"
	"github.com/wb-go/wbf/dbpg"
	"strconv"
	"time"
)

type ServerConfig struct {
	Port         string
	Name         string
	WriteTimeout time.Duration
}
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func BuildServerConfig(cfg *config.Config, log *zerolog.Logger) ServerConfig {
	port := cfg.GetString("server.port")
	serverName := cfg.GetString("server.name")
	writeTimeoutStr := cfg.GetString("server.write_timeout")

	writeTimeout, err := time.ParseDuration(writeTimeoutStr)
	if err != nil {
		log.Fatal().Msgf("invalid write_timeout value: %v", err)
	}

	log.Info().Msgf("Starting %s on port %s (timeout %s)", serverName, port, writeTimeout)

	return ServerConfig{
		Port:         port,
		Name:         serverName,
		WriteTimeout: writeTimeout,
	}
}
func BuildDBConfig(cfg *config.Config, log *zerolog.Logger) (string, []string, *dbpg.Options, error) {
	dbHost := cfg.GetString("database.host")
	dbPortStr := cfg.GetString("database.port")
	dbPort, err := strconv.Atoi(dbPortStr)
	if err != nil {
		log.Error().Msgf("invalid database.port: %v", err)
		return "", nil, nil, fmt.Errorf("invalid database.port: %w", err)
	}

	dbName := cfg.GetString("database.name")
	dbUser := cfg.GetString("database.user")
	dbPass := cfg.GetString("database.password")
	sslMode := cfg.GetString("database.ssl_mode")

	maxOpenConnsStr := cfg.GetString("database.max_conns")
	maxOpenConns, err := strconv.Atoi(maxOpenConnsStr)
	if err != nil {
		log.Error().Msgf("invalid database.max_conns: %v", err)
		return "", nil, nil, fmt.Errorf("invalid database.max_conns: %w", err)
	}

	maxIdleConnsStr := cfg.GetString("database.max_idle_conns")
	maxIdleConns, err := strconv.Atoi(maxIdleConnsStr)
	if err != nil {
		log.Error().Msgf("invalid database.max_idle_conns: %v", err)
		return "", nil, nil, fmt.Errorf("invalid database.max_idle_conns: %w", err)
	}

	connMaxLifetimeStr := cfg.GetString("database.max_conn_lifetime")
	connMaxLifetime, err := time.ParseDuration(connMaxLifetimeStr)
	if err != nil {
		log.Error().Msgf("invalid database.max_conn_lifetime: %v", err)
		return "", nil, nil, fmt.Errorf("invalid database.max_conn_lifetime: %w", err)
	}

	log.Info().Msgf("Database config: host=%s port=%d dbname=%s user=%s sslmode=%s",
		dbHost, dbPort, dbName, dbUser, sslMode)

	opts := &dbpg.Options{
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
	}

	masterDSN := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPass, dbName, sslMode,
	)
	slaveDSNs := []string{}

	log.Info().Msgf("Master DSN prepared, pool options: %+v", opts)

	return masterDSN, slaveDSNs, opts, nil
}

func BuildRedisConfig(cfg *config.Config, log *zerolog.Logger) (*RedisConfig, error) {
	addr := cfg.GetString("redis.addr")
	password := cfg.GetString("redis.password")
	dbStr := cfg.GetString("redis.db")

	db, err := strconv.Atoi(dbStr)
	if err != nil {
		log.Error().Msgf("invalid redis.db value: %v", err)
		return nil, fmt.Errorf("invalid redis.db value: %w", err)
	}

	log.Info().Msgf("Redis config loaded: %s, db=%d", addr, db)

	return &RedisConfig{
		Addr:     addr,
		Password: password,
		DB:       db,
	}, nil
}

type RabbitConfig struct {
	Url      string
	Exchange string
	Queue    string
}

func BuildRabbitConfig(cfg *config.Config, log *zerolog.Logger) (*RabbitConfig, error) {
	url := cfg.GetString("rabbitmq.url")
	exchange := cfg.GetString("rabbitmq.exchange")
	queue := cfg.GetString("rabbitmq.queue")

	if url == "" || exchange == "" || queue == "" {
		return nil, fmt.Errorf("incomplete rabbitmq configuration")
	}

	log.Info().Msgf("RabbitMQ config loaded: url=%s, exchange=%s, queue=%s", url, exchange, queue)
	return &RabbitConfig{
		Url:      url,
		Exchange: exchange,
		Queue:    queue,
	}, nil
}
