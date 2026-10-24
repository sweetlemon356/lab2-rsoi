package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sweetlemon356/lab2-rsoi/internal/payment/domain"
	db "github.com/sweetlemon356/lab2-rsoi/internal/platform/postgres"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Get(ctx context.Context, id string) (domain.Payment, error) {
	payment, err := readPayment(r.pool.QueryRow(ctx, "SELECT payment_uid::text,price,status FROM payment WHERE payment_uid=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, domain.ErrNotFound
	}
	return payment, err
}

func (r *Repository) Create(ctx context.Context, candidate domain.Payment) (domain.Payment, error) {
	tx, err := db.BeginOperation(ctx, r.pool, candidate.UID)
	if err != nil {
		return domain.Payment{}, err
	}
	defer db.Rollback(tx)
	var canceled bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM payment_cancellations WHERE payment_uid=$1)", candidate.UID).Scan(&canceled); err != nil {
		return domain.Payment{}, err
	}
	if canceled {
		return domain.Payment{}, domain.ErrCanceled
	}
	existing, err := readPayment(tx.QueryRow(ctx, "SELECT payment_uid::text,price,status FROM payment WHERE payment_uid=$1", candidate.UID))
	if err == nil {
		if err := existing.AcceptReplay(candidate); err != nil {
			return domain.Payment{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Payment{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Payment{}, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO payment(payment_uid,price,status) VALUES($1,$2,$3)", candidate.UID, candidate.Price, candidate.Status); err != nil {
		return domain.Payment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Payment{}, err
	}
	return candidate, nil
}

func (r *Repository) Cancel(ctx context.Context, id string) error {
	tx, err := db.BeginOperation(ctx, r.pool, id)
	if err != nil {
		return err
	}
	defer db.Rollback(tx)
	if _, err := tx.Exec(ctx, "INSERT INTO payment_cancellations(payment_uid) VALUES($1) ON CONFLICT DO NOTHING", id); err != nil {
		return err
	}
	existing, err := readPayment(tx.QueryRow(ctx, "SELECT payment_uid::text,price,status FROM payment WHERE payment_uid=$1", id))
	if err == nil {
		if _, err := tx.Exec(ctx, "UPDATE payment SET status=$2 WHERE payment_uid=$1", id, existing.Cancel().Status); err != nil {
			return err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	return tx.Commit(ctx)
}

func readPayment(row pgx.Row) (domain.Payment, error) {
	var id, status string
	var price int64
	if err := row.Scan(&id, &price, &status); err != nil {
		return domain.Payment{}, err
	}
	payment, err := domain.NewPayment(id, price)
	if err != nil {
		return domain.Payment{}, err
	}
	if status == "CANCELED" {
		payment = payment.Cancel()
	}
	return payment, nil
}
