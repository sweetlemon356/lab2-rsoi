package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address                                                        string
	ReservationURL, PaymentURL, LoyaltyURL                         string
	RequestTimeout, ClientTimeout, CleanupTimeout, ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{Address: environment("GATEWAY_ADDR", ":8080")}
	_, port, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return Config{}, fmt.Errorf("GATEWAY_ADDR must be host:port: %w", err)
	}
	if !validPort(port) {
		return Config{}, fmt.Errorf("GATEWAY_ADDR port must be between 1 and 65535")
	}
	for _, item := range []struct {
		key, fallback string
		target        *string
	}{
		{"RESERVATION_URL", "http://localhost:8070", &cfg.ReservationURL},
		{"PAYMENT_URL", "http://localhost:8060", &cfg.PaymentURL},
		{"LOYALTY_URL", "http://localhost:8050", &cfg.LoyaltyURL},
	} {
		value := environment(item.key, item.fallback)
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
			return Config{}, fmt.Errorf("%s must be an HTTP(S) URL without credentials, query or fragment", item.key)
		}
		if parsed.Port() != "" && !validPort(parsed.Port()) {
			return Config{}, fmt.Errorf("%s has an invalid port", item.key)
		}
		*item.target = strings.TrimRight(value, "/")
	}
	for _, item := range []struct {
		key      string
		fallback time.Duration
		target   *time.Duration
	}{
		{"REQUEST_TIMEOUT", 5 * time.Second, &cfg.RequestTimeout},
		{"CLIENT_TIMEOUT", 2 * time.Second, &cfg.ClientTimeout},
		{"CLEANUP_TIMEOUT", 2 * time.Second, &cfg.CleanupTimeout},
		{"SHUTDOWN_TIMEOUT", 10 * time.Second, &cfg.ShutdownTimeout},
	} {
		value, err := time.ParseDuration(environment(item.key, item.fallback.String()))
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("%s must be a positive duration", item.key)
		}
		*item.target = value
	}
	if cfg.ShutdownTimeout < cfg.CleanupTimeout {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be at least CLEANUP_TIMEOUT")
	}
	return cfg, nil
}

func environment(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func validPort(value string) bool {
	port, err := strconv.Atoi(value)
	return err == nil && port >= 1 && port <= 65535
}
