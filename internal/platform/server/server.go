package server

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BuildHandler func(*pgxpool.Pool, *slog.Logger, Config) http.Handler

// Main centralizes process lifecycle; service-specific dependency wiring stays in cmd.
func Main(defaultAddress string, tables []string, build BuildHandler) int {
	health := flag.Bool("healthcheck", false, "check this service readiness and exit")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := ReadConfig(defaultAddress)
	if err == nil {
		if *health {
			err = checkHealth(cfg.Address)
		} else {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			err = run(ctx, cfg, tables, build, logger)
		}
	}
	if err != nil {
		logger.Error("service failed", "error", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, cfg Config, tables []string, build BuildHandler, logger *slog.Logger) error {
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	poolConfig.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(startup, poolConfig)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(startup); err != nil {
		return fmt.Errorf("database startup: %w", err)
	}
	for _, table := range tables {
		var exists bool
		if err := pool.QueryRow(startup, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("missing table %s: apply migrations first", table)
		}
	}
	httpServer := &http.Server{
		Addr:              cfg.Address,
		Handler:           build(pool, logger, cfg),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.RequestTimeout,
		WriteTimeout:      cfg.RequestTimeout + time.Second,
		IdleTimeout:       60 * time.Second,
	}
	finished := make(chan error, 1)
	go func() { finished <- httpServer.ListenAndServe() }()
	logger.Info("service starting", "address", cfg.Address)
	select {
	case err := <-finished:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdown); err != nil {
			_ = httpServer.Close()
			return err
		}
		if err := <-finished; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		logger.Info("service stopped")
		return nil
	}
}

func checkHealth(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + net.JoinHostPort(host, port) + "/manage/health")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("health status %d", response.StatusCode)
	}
	return nil
}
