package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/server"
	httpadapter "github.com/sweetlemon356/lab2-rsoi/internal/reservation/adapters/http"
	"github.com/sweetlemon356/lab2-rsoi/internal/reservation/adapters/postgres"
	"github.com/sweetlemon356/lab2-rsoi/internal/reservation/application"
)

func main() {
	os.Exit(server.Main(":8070", []string{"hotels", "reservation", "reservation_cancellations"}, buildHandler))
}

func buildHandler(pool *pgxpool.Pool, logger *slog.Logger, cfg server.Config) http.Handler {
	repository := postgres.NewRepository(pool)
	useCase := application.NewService(repository, repository)
	return httpadapter.NewHandler(useCase, useCase, pool.Ping, cfg.RequestTimeout, logger)
}
