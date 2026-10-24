package main

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

	"github.com/google/uuid"
	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/adapters/clients"
	httpadapter "github.com/sweetlemon356/lab2-rsoi/internal/gateway/adapters/http"
	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/config"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check the local Gateway health endpoint and exit")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err == nil {
		if *healthcheck {
			err = checkHealth(cfg.Address)
		} else {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			err = run(ctx, cfg, logger)
		}
	}
	if err != nil {
		logger.Error("gateway failed", "error", err)
		os.Exit(1)
	}
}

func buildHandler(cfg config.Config, logger *slog.Logger, httpClient *http.Client) http.Handler {
	reservation := clients.NewReservationHTTPClient(cfg.ReservationURL, httpClient)
	payment := clients.NewPaymentHTTPClient(cfg.PaymentURL, httpClient)
	loyalty := clients.NewLoyaltyHTTPClient(cfg.LoyaltyURL, httpClient)
	newID := func() (string, error) { id, err := uuid.NewRandom(); return id.String(), err }
	service := application.NewService(reservation, payment, loyalty, newID, cfg.CleanupTimeout)

	hotels := httpadapter.NewHotelController(service)
	reservations := httpadapter.NewReservationController(service)
	loyaltyController := httpadapter.NewLoyaltyController(service)
	profile := httpadapter.NewProfileController(service)
	return httpadapter.NewRouter(
		hotels, reservations, loyaltyController, profile,
		cfg.RequestTimeout, logger,
	)
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	client := &http.Client{
		Timeout:       cfg.ClientTimeout,
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	server := &http.Server{
		Addr: cfg.Address, Handler: buildHandler(cfg, logger, client),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: cfg.RequestTimeout + cfg.CleanupTimeout + time.Second, IdleTimeout: 60 * time.Second,
	}
	finished := make(chan error, 1)
	go func() { finished <- server.ListenAndServe() }()
	logger.Info("gateway starting", "address", cfg.Address)
	select {
	case err := <-finished:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return fmt.Errorf("shutdown: %w", err)
		}
		if err := <-finished; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		logger.Info("gateway stopped")
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
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health status %d", response.StatusCode)
	}
	return nil
}
