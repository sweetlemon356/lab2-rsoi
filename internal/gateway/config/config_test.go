package config_test

import (
	"testing"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/config"
)

func cleanEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"GATEWAY_ADDR", "RESERVATION_URL", "PAYMENT_URL", "LOYALTY_URL", "REQUEST_TIMEOUT", "CLIENT_TIMEOUT", "CLEANUP_TIMEOUT", "SHUTDOWN_TIMEOUT"} {
		t.Setenv(key, "")
	}
}

func TestDefaultsProvideLocalDevelopmentConfiguration(t *testing.T) {
	cleanEnvironment(t)
	got, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Address != ":8080" || got.ReservationURL != "http://localhost:8070" || got.PaymentURL != "http://localhost:8060" || got.LoyaltyURL != "http://localhost:8050" {
		t.Fatalf("defaults=%+v", got)
	}
	if got.RequestTimeout != 5*time.Second || got.ClientTimeout != 2*time.Second || got.CleanupTimeout != 2*time.Second || got.ShutdownTimeout != 10*time.Second {
		t.Fatalf("timeouts=%+v", got)
	}
}

func TestEnvironmentOverridesDefaults(t *testing.T) {
	cleanEnvironment(t)
	t.Setenv("GATEWAY_ADDR", "127.0.0.1:9080")
	t.Setenv("PAYMENT_URL", "https://payment.example:8443/api/")
	t.Setenv("REQUEST_TIMEOUT", "750ms")
	got, err := config.Load()
	if err != nil || got.Address != "127.0.0.1:9080" || got.PaymentURL != "https://payment.example:8443/api" || got.RequestTimeout != 750*time.Millisecond {
		t.Fatalf("config=%+v, err=%v", got, err)
	}
}

func TestInvalidConfigurationFailsBeforeServerStarts(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"PAYMENT_URL", "payment:8060"}, {"PAYMENT_URL", "ftp://payment"}, {"PAYMENT_URL", "http://"},
		{"PAYMENT_URL", "http://user:password@payment"}, {"PAYMENT_URL", "http://payment?x=1"}, {"PAYMENT_URL", "http://payment#fragment"},
		{"GATEWAY_ADDR", "not-an-address"}, {"GATEWAY_ADDR", ":99999"}, {"GATEWAY_ADDR", ":0"},
		{"REQUEST_TIMEOUT", "0s"}, {"CLIENT_TIMEOUT", "-1s"}, {"CLEANUP_TIMEOUT", "forever"}, {"SHUTDOWN_TIMEOUT", "-1"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			cleanEnvironment(t)
			t.Setenv(tc.key, tc.value)
			if _, err := config.Load(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
