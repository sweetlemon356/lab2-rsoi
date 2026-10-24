package server

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address, DatabaseURL            string
	RequestTimeout, ShutdownTimeout time.Duration
}

func ReadConfig(defaultAddress string) (Config, error) {
	cfg := Config{Address: env("HTTP_ADDR", defaultAddress), DatabaseURL: os.Getenv("DATABASE_URL")}
	_, port, err := net.SplitHostPort(cfg.Address)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR must be host:port")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR has an invalid port")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" || parsed.Path == "" || parsed.Path == "/" {
		return Config{}, fmt.Errorf("DATABASE_URL must identify a PostgreSQL database")
	}
	for _, item := range []struct {
		key, fallback string
		target        *time.Duration
	}{{"REQUEST_TIMEOUT", "5s", &cfg.RequestTimeout}, {"SHUTDOWN_TIMEOUT", "10s", &cfg.ShutdownTimeout}} {
		value, err := time.ParseDuration(env(item.key, item.fallback))
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("%s must be a positive duration", item.key)
		}
		*item.target = value
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
