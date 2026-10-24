package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	httpadapter "github.com/sweetlemon356/lab2-rsoi/internal/payment/adapters/http"
	"github.com/sweetlemon356/lab2-rsoi/internal/payment/adapters/postgres"
	"github.com/sweetlemon356/lab2-rsoi/internal/payment/application"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/server"
)

func main() {
	os.Exit(server.Main(":8060", []string{"payment", "payment_cancellations"}, buildHandler))
}

func buildHandler(pool *pgxpool.Pool, logger *slog.Logger, cfg server.Config) http.Handler {
	repository := postgres.NewRepository(pool)
	useCase := application.NewService(repository)
	return httpadapter.NewHandler(useCase, pool.Ping, cfg.RequestTimeout, logger)
}
