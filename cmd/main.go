package main

import (
	"context"
	"fifthOne/internal/api/api"
	rabbitReader "fifthOne/internal/consumerWorker"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"fifthOne/cmd/buildCFG"

	"fifthOne/internal/rabbit"
	"fifthOne/internal/repo"

	"fifthOne/internal/service"

	"github.com/wb-go/wbf/config"
	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/zlog"
)

func main() {
	zlog.Init()
	log := zlog.Logger
	log.Info().Msg("Hello from zlog")

	cfg := config.New()
	if err := cfg.Load("config.yaml", "", "'"); err != nil {
		log.Fatal().Msgf("failed to load configuration: %v", err)
	}
	serverCfg := buildCFG.BuildServerConfig(cfg, &log)
	port := serverCfg.Port

	masterDSN, slaveDSNs, poolOptions, err := buildCFG.BuildDBConfig(cfg, &log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build DB config")
	}
	db, err := dbpg.New(masterDSN, slaveDSNs, poolOptions)
	if err != nil {
		log.Fatal().Msgf("failed to connect to DB: %v", err)
	}
	if err := db.Master.Ping(); err != nil {
		log.Fatal().Msgf("DB ping failed: %v", err)
	}
	log.Info().Msg("Database connected successfully")

	repository, err := repo.NewRepository(db, &log)
	if err != nil {
		log.Fatal().Msgf("failed to initialize repository: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get working directory")
	}
	migrationPath := filepath.Join(cwd, "migrations/postgres")
	if err := repository.MigrateUp(migrationPath); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}
	log.Info().Msg("Migrations applied successfully")

	rabbitCfg, err := buildCFG.BuildRabbitConfig(cfg, &log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load RabbitMQ config")
	}
	rmq, err := rabbit.NewRabbit(rabbitCfg.Url, rabbitCfg.Exchange, rabbitCfg.Queue)
	if err != nil {
		log.Fatal().Msgf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rmq.Close()

	// было тут

	workerCtx, cancelWorkers := context.WithCancel(context.Background())

	rabbitReaderer := rabbitReader.NewReader(rmq, repository)
	go rabbitReaderer.Start(workerCtx)

	serviceInstance := service.NewService(repository, &log, rmq)
	app := api.NewRouters(&api.Routers{Service: serviceInstance})

	serverErrChan := make(chan error, 1)
	go func() {
		log.Info().Msgf("Starting server on %s", port)
		if err := app.Run(":" + port); err != nil {
			serverErrChan <- fmt.Errorf("failed to start server: %w", err)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-signalChan:
		log.Info().Msgf("Received signal %s. Initiating shutdown...", sig)
	case err := <-serverErrChan:
		log.Error().Msgf("Server error: %v", err)
	}

	cancelWorkers()
	rabbitReaderer.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if closer, ok := interface{}(app).(interface{ Close(context.Context) error }); ok {
		if err := closer.Close(shutdownCtx); err != nil {
			log.Error().Msgf("Error shutting down server: %v", err)
		}
	}

	log.Info().Msg("Rolling back migrations...")
	if err := repository.MigrateDown(migrationPath); err != nil {
		log.Fatal().Msgf("failed to rollback migrations: %v", err)
	}
	log.Info().Msg("Migrations rolled back successfully")
	log.Info().Msg("Shutdown complete")
}
