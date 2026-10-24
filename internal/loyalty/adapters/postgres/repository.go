package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sweetlemon356/lab2-rsoi/internal/loyalty/domain"
	db "github.com/sweetlemon356/lab2-rsoi/internal/platform/postgres"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Get(ctx context.Context, user string) (domain.Account, error) {
	if _, err := r.pool.Exec(ctx, "INSERT INTO loyalty(username) VALUES($1) ON CONFLICT DO NOTHING", user); err != nil {
		return domain.Account{}, err
	}
	var count int
	if err := r.pool.QueryRow(ctx, "SELECT reservation_count FROM loyalty WHERE username=$1", user).Scan(&count); err != nil {
		return domain.Account{}, err
	}
	return domain.NewAccount(user, count)
}

func (r *Repository) ApplyBooking(ctx context.Context, user, id string, action domain.Action) (domain.Account, error) {
	tx, err := db.BeginOperation(ctx, r.pool, id)
	if err != nil {
		return domain.Account{}, err
	}
	defer db.Rollback(tx)
	if _, err := tx.Exec(ctx, "INSERT INTO loyalty(username) VALUES($1) ON CONFLICT DO NOTHING", user); err != nil {
		return domain.Account{}, err
	}
	var count int
	if err := tx.QueryRow(ctx, "SELECT reservation_count FROM loyalty WHERE username=$1 FOR UPDATE", user).Scan(&count); err != nil {
		return domain.Account{}, err
	}
	account, err := domain.NewAccount(user, count)
	if err != nil {
		return domain.Account{}, err
	}
	var owner string
	var current domain.BookingState
	err = tx.QueryRow(ctx, "SELECT username,status FROM loyalty_reservations WHERE reservation_uid=$1", id).Scan(&owner, &current)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, err
	}
	if err == nil && owner != user {
		return domain.Account{}, domain.ErrConflict
	}
	next, delta, err := domain.Transition(current, action)
	if err != nil {
		return domain.Account{}, err
	}
	account, err = account.WithDelta(delta)
	if err != nil {
		return domain.Account{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO loyalty_reservations(reservation_uid,username,status) VALUES($1,$2,$3)
 ON CONFLICT(reservation_uid) DO UPDATE SET status=EXCLUDED.status`, id, user, next); err != nil {
		return domain.Account{}, err
	}
	if _, err := tx.Exec(ctx, "UPDATE loyalty SET reservation_count=$2,status=$3,discount=$4 WHERE username=$1", user, account.ReservationCount, account.Status, account.Discount); err != nil {
		return domain.Account{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Account{}, err
	}
	return account, nil
}
