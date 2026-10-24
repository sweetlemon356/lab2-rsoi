package server_test

import (
	"testing"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/platform/server"
)

func clean(t *testing.T) {
	t.Helper()
	for _, key := range []string{"HTTP_ADDR", "DATABASE_URL", "REQUEST_TIMEOUT", "SHUTDOWN_TIMEOUT"} {
		t.Setenv(key, "")
	}
}

func TestServiceConfigRequiresDatabaseAndValidatesTimeouts(t *testing.T) {
	clean(t)
	if _, err := server.ReadConfig(":8060"); err == nil {
		t.Fatal("missing database accepted")
	}
	t.Setenv("DATABASE_URL", "postgres://user:test@localhost/payment")
	cfg, err := server.ReadConfig(":8060")
	if err != nil || cfg.Address != ":8060" || cfg.RequestTimeout != 5*time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("config=%+v,err=%v", cfg, err)
	}
	for _, tc := range []struct{ key, value string }{{"HTTP_ADDR", ":0"}, {"HTTP_ADDR", "bad"}, {"REQUEST_TIMEOUT", "0"}, {"SHUTDOWN_TIMEOUT", "-1s"}, {"DATABASE_URL", "http://localhost/db"}} {
		t.Run(tc.key, func(t *testing.T) {
			clean(t)
			t.Setenv("DATABASE_URL", "postgres://user:test@localhost/payment")
			t.Setenv(tc.key, tc.value)
			if _, err := server.ReadConfig(":8060"); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
