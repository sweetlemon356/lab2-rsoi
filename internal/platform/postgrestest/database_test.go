package postgrestest

import "testing"

func TestGuardChecksEffectiveDatabaseIncludingQueryOverrides(t *testing.T) {
	for _, dsn := range []string{"postgres://localhost/payment", "postgres://localhost/payment_test?dbname=payment", "postgres://localhost/payment_test?database=payment"} {
		if _, err := databaseConfig(dsn); err == nil {
			t.Fatalf("unsafe DSN accepted: %s", dsn)
		}
	}
	if _, err := databaseConfig("postgres://localhost/payment_test"); err != nil {
		t.Fatal(err)
	}
}
