package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/sweetlemon356/lab2-rsoi/internal/platform/postgres"
	"github.com/sweetlemon356/lab2-rsoi/internal/reservation/application"
	"github.com/sweetlemon356/lab2-rsoi/internal/reservation/domain"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const hotelColumns = "hotel_uid::text,name,country,city,address,COALESCE(stars,0),price"
const detailsQuery = `SELECT r.reservation_uid::text,r.username,r.payment_uid::text,r.status,r.start_date,r.end_date,
 h.hotel_uid::text,h.name,h.country,h.city,h.address,COALESCE(h.stars,0),h.price
 FROM reservation r JOIN hotels h ON h.id=r.hotel_id`

func (r *Repository) List(ctx context.Context, q application.PageQuery) (application.HotelPage, error) {
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM hotels").Scan(&total); err != nil {
		return application.HotelPage{}, err
	}
	page := q.Page
	if page == 0 {
		page = 1
	}
	rows, err := r.pool.Query(ctx, "SELECT "+hotelColumns+" FROM hotels ORDER BY id LIMIT $1 OFFSET $2", q.Size, int64(page-1)*int64(q.Size))
	if err != nil {
		return application.HotelPage{}, err
	}
	defer rows.Close()
	items := []domain.Hotel{}
	for rows.Next() {
		hotel, err := readHotel(rows)
		if err != nil {
			return application.HotelPage{}, err
		}
		items = append(items, hotel)
	}
	if err := rows.Err(); err != nil {
		return application.HotelPage{}, err
	}
	return application.HotelPage{Page: q.Page, PageSize: q.Size, TotalElements: total, Items: items}, nil
}

func (r *Repository) GetHotel(ctx context.Context, id string) (domain.Hotel, error) {
	hotel, err := readHotel(r.pool.QueryRow(ctx, "SELECT "+hotelColumns+" FROM hotels WHERE hotel_uid=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Hotel{}, domain.ErrNotFound
	}
	return hotel, err
}

func (r *Repository) ListByUser(ctx context.Context, user string) ([]application.Details, error) {
	rows, err := r.pool.Query(ctx, detailsQuery+" WHERE r.username=$1 ORDER BY r.id", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []application.Details{}
	for rows.Next() {
		value, err := readDetails(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *Repository) GetByUser(ctx context.Context, user, id string) (application.Details, error) {
	value, err := readDetails(r.pool.QueryRow(ctx, detailsQuery+" WHERE r.reservation_uid=$1 AND r.username=$2", id, user))
	if errors.Is(err, pgx.ErrNoRows) {
		return application.Details{}, domain.ErrNotFound
	}
	return value, err
}

func (r *Repository) Create(ctx context.Context, candidate domain.Reservation) error {
	tx, err := db.BeginOperation(ctx, r.pool, candidate.UID)
	if err != nil {
		return err
	}
	defer db.Rollback(tx)
	var canceled bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM reservation_cancellations WHERE reservation_uid=$1)", candidate.UID).Scan(&canceled); err != nil {
		return err
	}
	if canceled {
		return domain.ErrCanceled
	}
	existing, err := readDetails(tx.QueryRow(ctx, detailsQuery+" WHERE r.reservation_uid=$1", candidate.UID))
	if err == nil {
		if err := existing.Reservation.AcceptReplay(candidate); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	result, err := tx.Exec(ctx, `INSERT INTO reservation(reservation_uid,username,payment_uid,hotel_id,status,start_date,end_date)
 SELECT $1,$2,$3,id,$5,$6,$7 FROM hotels WHERE hotel_uid=$4`, candidate.UID, candidate.Username, candidate.PaymentUID, candidate.HotelUID, candidate.Status, candidate.Stay.StartDate, candidate.Stay.EndDate)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r *Repository) Cancel(ctx context.Context, user, id string) error {
	tx, err := db.BeginOperation(ctx, r.pool, id)
	if err != nil {
		return err
	}
	defer db.Rollback(tx)
	var owner string
	err = tx.QueryRow(ctx, "SELECT username FROM reservation_cancellations WHERE reservation_uid=$1", id).Scan(&owner)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil && owner != user {
		return domain.ErrNotFound
	}
	existing, err := readDetails(tx.QueryRow(ctx, detailsQuery+" WHERE r.reservation_uid=$1", id))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil {
		if existing.Reservation.Username != user {
			return domain.ErrNotFound
		}
		if _, err := tx.Exec(ctx, "UPDATE reservation SET status=$2 WHERE reservation_uid=$1", id, existing.Reservation.Cancel().Status); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, "INSERT INTO reservation_cancellations(reservation_uid,username) VALUES($1,$2) ON CONFLICT DO NOTHING", id, user); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func readHotel(row pgx.Row) (domain.Hotel, error) {
	var id, name, country, city, address string
	var stars int
	var price int64
	if err := row.Scan(&id, &name, &country, &city, &address, &stars, &price); err != nil {
		return domain.Hotel{}, err
	}
	return domain.Hotel{UID: id, Name: name, Country: country, City: city, Address: address, Stars: stars, Price: price}, nil
}

func readDetails(row pgx.Row) (application.Details, error) {
	var id, user, payment, status, hotelID, name, country, city, address string
	var start, end time.Time
	var stars int
	var price int64
	if err := row.Scan(&id, &user, &payment, &status, &start, &end, &hotelID, &name, &country, &city, &address, &stars, &price); err != nil {
		return application.Details{}, err
	}
	stay, err := domain.NewStay(start, end)
	if err != nil {
		return application.Details{}, err
	}
	reservation, err := domain.NewReservation(id, user, payment, hotelID, stay)
	if err != nil {
		return application.Details{}, err
	}
	if status == "CANCELED" {
		reservation = reservation.Cancel()
	}
	return application.Details{Reservation: reservation, Hotel: domain.Hotel{UID: hotelID, Name: name, Country: country, City: city, Address: address, Stars: stars, Price: price}}, nil
}
