package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	p "github.com/sweetlemon356/lab2-rsoi/internal/loyalty/adapters/postgres"
	d "github.com/sweetlemon356/lab2-rsoi/internal/loyalty/domain"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/postgrestest"
)

func TestLoyaltyLedgerPreservesSeededCountAndTerminalCancel(t *testing.T) {
	pool := postgrestest.Open(t, "LOYALTY_TEST_DATABASE_URL")
	r := p.NewRepository(pool)
	ctx := context.Background()
	user := uuid.NewString()
	account, err := r.Get(ctx, user)
	if err != nil || account.ReservationCount != 0 || account.Discount != 5 {
		t.Fatalf("account=%+v,err=%v", account, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE loyalty SET reservation_count=25,status='GOLD',discount=10 WHERE username=$1", user); err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	for range 2 {
		account, err = r.ApplyBooking(ctx, user, id, d.AddBooking)
		if err != nil || account.ReservationCount != 26 {
			t.Fatalf("account=%+v,err=%v", account, err)
		}
	}
	for range 2 {
		account, err = r.ApplyBooking(ctx, user, id, d.CancelBooking)
		if err != nil || account.ReservationCount != 25 {
			t.Fatalf("account=%+v,err=%v", account, err)
		}
	}
	if _, err := r.ApplyBooking(ctx, user, id, d.AddBooking); !errors.Is(err, d.ErrCanceled) {
		t.Fatalf("late add=%v", err)
	}
	id = uuid.NewString()
	if _, err := r.ApplyBooking(ctx, user, id, d.CancelBooking); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ApplyBooking(ctx, user, id, d.AddBooking); !errors.Is(err, d.ErrCanceled) {
		t.Fatalf("cancel-before-add=%v", err)
	}
	other := uuid.NewString()
	if _, err := r.ApplyBooking(ctx, other, id, d.CancelBooking); !errors.Is(err, d.ErrConflict) {
		t.Fatalf("foreign booking accepted: %v", err)
	}
}

func TestLoyaltyConcurrentAddAndCancelCannotResurrectBooking(t *testing.T) {
	pool := postgrestest.Open(t, "LOYALTY_TEST_DATABASE_URL")
	r := p.NewRepository(pool)
	ctx := context.Background()
	user, id := uuid.NewString(), uuid.NewString()
	var wg sync.WaitGroup
	var addErr, cancelErr error
	wg.Add(2)
	go func() { defer wg.Done(); _, addErr = r.ApplyBooking(ctx, user, id, d.AddBooking) }()
	go func() { defer wg.Done(); _, cancelErr = r.ApplyBooking(ctx, user, id, d.CancelBooking) }()
	wg.Wait()
	if cancelErr != nil || (addErr != nil && !errors.Is(addErr, d.ErrCanceled)) {
		t.Fatalf("add=%v,cancel=%v", addErr, cancelErr)
	}
	account, err := r.Get(ctx, user)
	if err != nil || account.ReservationCount != 0 {
		t.Fatalf("account=%+v,err=%v", account, err)
	}
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM loyalty_reservations WHERE reservation_uid=$1", id).Scan(&status); err != nil || status != "CANCELED" {
		t.Fatalf("status=%s,err=%v", status, err)
	}
	if _, err := r.ApplyBooking(ctx, user, id, d.AddBooking); !errors.Is(err, d.ErrCanceled) {
		t.Fatalf("late add=%v", err)
	}
}

func TestSQLFailureRollsBackAccountAndLedger(t *testing.T) {
	for _, table := range []string{"loyalty", "loyalty_reservations"} {
		t.Run(table, func(t *testing.T) {
			pool := postgrestest.Open(t, "LOYALTY_TEST_DATABASE_URL")
			r := p.NewRepository(pool)
			ctx := context.Background()
			user, id := uuid.NewString(), uuid.NewString()
			before, err := r.ApplyBooking(ctx, user, id, d.AddBooking)
			if err != nil {
				t.Fatal(err)
			}
			name := "test_fail_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			function := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.username='%s' THEN RAISE EXCEPTION 'injected test failure'; END IF; RETURN NEW; END $$`, name, user)
			if _, err := pool.Exec(ctx, function); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_, _ = pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS "+name+" ON "+table)
				_, _ = pool.Exec(context.Background(), "DROP FUNCTION IF EXISTS "+name+"()")
			})
			if _, err := pool.Exec(ctx, "CREATE TRIGGER "+name+" BEFORE UPDATE ON "+table+" FOR EACH ROW EXECUTE FUNCTION "+name+"()"); err != nil {
				t.Fatal(err)
			}
			if _, err := r.ApplyBooking(ctx, user, id, d.CancelBooking); err == nil {
				t.Fatal("injected SQL failure ignored")
			}
			after, err := r.Get(ctx, user)
			if err != nil || after != before {
				t.Fatalf("account changed: before=%+v,after=%+v,err=%v", before, after, err)
			}
			var count, discount int
			var status, ledger string
			if err := pool.QueryRow(ctx, "SELECT reservation_count,status,discount FROM loyalty WHERE username=$1", user).Scan(&count, &status, &discount); err != nil || count != before.ReservationCount || status != before.Status || discount != before.Discount {
				t.Fatalf("stored account changed: %d/%s/%d,err=%v", count, status, discount, err)
			}
			if err := pool.QueryRow(ctx, "SELECT status FROM loyalty_reservations WHERE reservation_uid=$1", id).Scan(&ledger); err != nil || ledger != "ACTIVE" {
				t.Fatalf("ledger=%s,err=%v", ledger, err)
			}
		})
	}
}

func TestConcurrentLoyaltyUpdatesRemainAtomicAndIdempotent(t *testing.T) {
	pool := postgrestest.Open(t, "LOYALTY_TEST_DATABASE_URL")
	r := p.NewRepository(pool)
	ctx := context.Background()
	user := uuid.NewString()
	ids := make([]string, 20)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	var wg sync.WaitGroup
	results := make(chan error, 40)
	for _, id := range ids {
		for range 2 {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := r.ApplyBooking(ctx, user, id, d.AddBooking); results <- err }()
		}
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	account, err := r.Get(ctx, user)
	if err != nil || account.ReservationCount != 20 || account.Status != "GOLD" || account.Discount != 10 {
		t.Fatalf("account=%+v,err=%v", account, err)
	}
	results = make(chan error, 22)
	for _, id := range ids[:11] {
		for range 2 {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := r.ApplyBooking(ctx, user, id, d.CancelBooking); results <- err }()
		}
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	account, err = r.Get(ctx, user)
	if err != nil || account.ReservationCount != 9 || account.Status != "BRONZE" || account.Discount != 5 {
		t.Fatalf("account=%+v,err=%v", account, err)
	}
	var count, discount int
	var status string
	if err := pool.QueryRow(ctx, "SELECT reservation_count,status,discount FROM loyalty WHERE username=$1", user).Scan(&count, &status, &discount); err != nil || count != 9 || status != "BRONZE" || discount != 5 {
		t.Fatalf("stored=%d/%s/%d,err=%v", count, status, discount, err)
	}
}

func TestFailedLoyaltyTransitionRollsBackLedger(t *testing.T) {
	pool := postgrestest.Open(t, "LOYALTY_TEST_DATABASE_URL")
	r := p.NewRepository(pool)
	ctx := context.Background()
	user := uuid.NewString()
	id := uuid.NewString()
	if _, err := r.ApplyBooking(ctx, user, id, d.AddBooking); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE loyalty SET reservation_count=0 WHERE username=$1", user); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ApplyBooking(ctx, user, id, d.CancelBooking); !errors.Is(err, d.ErrInvalid) {
		t.Fatalf("underflow=%v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM loyalty_reservations WHERE reservation_uid=$1", id).Scan(&status); err != nil || status != "ACTIVE" {
		t.Fatalf("ledger=%s,err=%v", status, err)
	}
}
