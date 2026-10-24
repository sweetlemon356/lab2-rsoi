package application_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	app "github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

const (
	username      = "Test Max"
	hotelID       = "049161bb-badd-4fa8-9d90-87c9a82b0668"
	reservationID = "9b4ba1f7-e5ac-465b-ace4-7b54dec20f9a"
	paymentID     = "5c9ced89-f43a-4ac4-a516-e6660e5bddcb"
)

var (
	startDate = time.Date(2021, 10, 8, 0, 0, 0, 0, time.UTC)
	endDate   = time.Date(2021, 10, 11, 0, 0, 0, 0, time.UTC)
)

type reservationFake struct {
	hotels func(context.Context, app.PageQuery) (app.HotelPage, error)
	list   func(context.Context, string) ([]app.ReservationData, error)
	get    func(context.Context, string, string) (app.ReservationData, error)
	quote  func(context.Context, app.QuoteRequest) (app.ReservationQuote, error)
	create func(context.Context, app.SaveReservationCommand) error
	cancel func(context.Context, string, string) error
}

func (f reservationFake) ListHotels(ctx context.Context, query app.PageQuery) (app.HotelPage, error) {
	return f.hotels(ctx, query)
}

func (f reservationFake) ListByUser(ctx context.Context, u string) ([]app.ReservationData, error) {
	return f.list(ctx, u)
}
func (f reservationFake) GetByUser(ctx context.Context, u, id string) (app.ReservationData, error) {
	return f.get(ctx, u, id)
}
func (f reservationFake) Quote(ctx context.Context, q app.QuoteRequest) (app.ReservationQuote, error) {
	return f.quote(ctx, q)
}
func (f reservationFake) Create(ctx context.Context, c app.SaveReservationCommand) error {
	return f.create(ctx, c)
}
func (f reservationFake) Cancel(ctx context.Context, u, id string) error { return f.cancel(ctx, u, id) }

type paymentFake struct {
	get    func(context.Context, string) (app.PaymentInfo, error)
	create func(context.Context, app.CreatePaymentCommand) (app.PaymentInfo, error)
	cancel func(context.Context, string) error
}

func (f paymentFake) Get(ctx context.Context, id string) (app.PaymentInfo, error) {
	return f.get(ctx, id)
}
func (f paymentFake) Create(ctx context.Context, c app.CreatePaymentCommand) (app.PaymentInfo, error) {
	return f.create(ctx, c)
}
func (f paymentFake) Cancel(ctx context.Context, id string) error { return f.cancel(ctx, id) }

type loyaltyFake struct {
	get    func(context.Context, string) (app.LoyaltyInfo, error)
	add    func(context.Context, string, string) error
	remove func(context.Context, string, string) error
}

func (f loyaltyFake) Get(ctx context.Context, u string) (app.LoyaltyInfo, error) {
	return f.get(ctx, u)
}
func (f loyaltyFake) AddReservation(ctx context.Context, u, id string) error {
	return f.add(ctx, u, id)
}
func (f loyaltyFake) RemoveReservation(ctx context.Context, u, id string) error {
	return f.remove(ctx, u, id)
}

func newService(reservations reservationFake, payments paymentFake, loyalty loyaltyFake) *app.Service {
	return app.NewService(reservations, payments, loyalty, nil, time.Second)
}

func reservation() app.ReservationData {
	return app.ReservationData{ReservationUID: reservationID, Username: username, PaymentUID: paymentID,
		Hotel:     app.HotelInfo{HotelUID: hotelID, Name: "Ararat Park Hyatt Moscow", FullAddress: "Россия, Москва, Неглинная ул., 4", Stars: 5},
		StartDate: startDate, EndDate: endDate, Status: "PAID"}
}

func TestListHotelsPassesPaginationAndNormalizesEmptyItems(t *testing.T) {
	query := app.PageQuery{Page: 2, Size: 10}
	s := newService(reservationFake{hotels: func(_ context.Context, got app.PageQuery) (app.HotelPage, error) {
		if got != query {
			t.Fatalf("query=%+v, want %+v", got, query)
		}
		return app.HotelPage{Page: 2, PageSize: 10}, nil
	}}, paymentFake{}, loyaltyFake{})
	got, err := s.ListHotels(context.Background(), query)
	if err != nil || got.Items == nil || got.Page != 2 {
		t.Fatalf("result=%+v, err=%v", got, err)
	}
}

func TestGetUserInfoCombinesReservationsPaymentsAndLoyalty(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	first := reservation()
	second := first
	second.ReservationUID = "064825bc-7062-4f0c-af52-a586c4b3274e"
	second.PaymentUID = "0dc29fbb-851e-4166-b242-cf2f65f049d7"
	payments := map[string]app.PaymentInfo{first.PaymentUID: {Status: "PAID", Price: 27000}, second.PaymentUID: {Status: "CANCELED", Price: 19000}}
	r := reservationFake{list: func(got context.Context, u string) ([]app.ReservationData, error) {
		if got.Value(contextKey{}) != "request" || u != username {
			t.Fatal("request context or username was lost")
		}
		return []app.ReservationData{first, second}, nil
	}}
	p := paymentFake{get: func(got context.Context, id string) (app.PaymentInfo, error) {
		if got.Value(contextKey{}) != "request" {
			t.Fatal("payment context was lost")
		}
		return payments[id], nil
	}}
	wantLoyalty := app.LoyaltyInfo{Status: "GOLD", Discount: 10, ReservationCount: 25}
	l := loyaltyFake{get: func(got context.Context, u string) (app.LoyaltyInfo, error) {
		if got.Value(contextKey{}) != "request" || u != username {
			t.Fatal("loyalty context or username was lost")
		}
		return wantLoyalty, nil
	}}
	got, err := newService(r, p, l).GetUserInfo(ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Reservations) != 2 || got.Loyalty != wantLoyalty {
		t.Fatalf("result=%+v", got)
	}
	for i, source := range []app.ReservationData{first, second} {
		view := got.Reservations[i]
		if view.ReservationUID != source.ReservationUID || view.Hotel != source.Hotel || !view.StartDate.Equal(startDate) || !view.EndDate.Equal(endDate) || view.Payment != payments[source.PaymentUID] {
			t.Fatalf("view=%+v", view)
		}
	}
}

func TestGetUserInfoEmptyReservationsStillReturnsLoyalty(t *testing.T) {
	r := reservationFake{list: func(context.Context, string) ([]app.ReservationData, error) { return nil, nil }}
	l := loyaltyFake{get: func(context.Context, string) (app.LoyaltyInfo, error) {
		return app.LoyaltyInfo{Status: "BRONZE", Discount: 5}, nil
	}}
	got, err := newService(r, paymentFake{}, l).GetUserInfo(context.Background(), username)
	if err != nil || got.Reservations == nil || len(got.Reservations) != 0 || got.Loyalty.Discount != 5 {
		t.Fatalf("result=%+v, err=%v", got, err)
	}
}

func TestQueriesDoNotHideDependencyFailures(t *testing.T) {
	for _, stage := range []string{"reservations", "payment", "loyalty"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("upstream unavailable")
			r := reservationFake{list: func(context.Context, string) ([]app.ReservationData, error) {
				if stage == "reservations" {
					return nil, failure
				}
				return []app.ReservationData{reservation()}, nil
			}}
			p := paymentFake{get: func(context.Context, string) (app.PaymentInfo, error) {
				if stage == "payment" {
					return app.PaymentInfo{}, failure
				}
				return app.PaymentInfo{Status: "PAID", Price: 27000}, nil
			}}
			l := loyaltyFake{get: func(context.Context, string) (app.LoyaltyInfo, error) {
				if stage == "loyalty" {
					return app.LoyaltyInfo{}, failure
				}
				return app.LoyaltyInfo{Status: "GOLD", Discount: 10}, nil
			}}
			_, err := newService(r, p, l).GetUserInfo(context.Background(), username)
			if !errors.Is(err, failure) {
				t.Fatalf("err=%v, want original failure", err)
			}
		})
	}
}

func TestReservationQueriesRejectForeignDataBeforeReadingPayment(t *testing.T) {
	foreign := reservation()
	foreign.Username = "Another user"
	r := reservationFake{
		list: func(context.Context, string) ([]app.ReservationData, error) {
			return []app.ReservationData{foreign}, nil
		},
		get: func(context.Context, string, string) (app.ReservationData, error) { return foreign, nil },
	}
	p := paymentFake{get: func(context.Context, string) (app.PaymentInfo, error) {
		t.Fatal("read payment for foreign booking")
		return app.PaymentInfo{}, nil
	}}
	if _, err := newService(r, p, loyaltyFake{}).GetReservation(context.Background(), username, reservationID); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("get error=%v", err)
	}
	if _, err := newService(r, p, loyaltyFake{}).ListReservations(context.Background(), username); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("list error=%v", err)
	}
}

func TestGetReservationCombinesOwnedBookingAndPayment(t *testing.T) {
	r := reservationFake{get: func(_ context.Context, u, id string) (app.ReservationData, error) {
		if u != username || id != reservationID {
			t.Fatal("incorrect ownership query")
		}
		return reservation(), nil
	}}
	p := paymentFake{get: func(_ context.Context, id string) (app.PaymentInfo, error) {
		if id != paymentID {
			t.Fatal("incorrect payment reference")
		}
		return app.PaymentInfo{Status: "PAID", Price: 27000}, nil
	}}
	got, err := newService(r, p, loyaltyFake{}).GetReservation(context.Background(), username, reservationID)
	if err != nil || got.ReservationUID != reservationID || got.Payment.Price != 27000 {
		t.Fatalf("result=%+v, err=%v", got, err)
	}
}

func TestCanceledContextStopsBeforeDependencyCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := reservationFake{list: func(context.Context, string) ([]app.ReservationData, error) {
		t.Fatal("called dependency after cancellation")
		return nil, nil
	}}
	_, err := newService(r, paymentFake{}, loyaltyFake{}).ListReservations(ctx, username)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

type bookingFixture struct {
	t                    *testing.T
	r                    reservationFake
	p                    paymentFake
	l                    loyaltyFake
	fail                 string
	failure              error
	cleanupFailure       error
	cancelRequest        context.CancelFunc
	paid, saved, counted bool
	cleanup              []string
	idCalls              int
}

func newBookingFixture(t *testing.T) *bookingFixture {
	f := &bookingFixture{t: t, failure: errors.New("write response lost")}
	f.r = reservationFake{
		quote: func(_ context.Context, q app.QuoteRequest) (app.ReservationQuote, error) {
			if q.HotelUID != hotelID || q.Discount != 5 || !q.StartDate.Equal(startDate) || !q.EndDate.Equal(endDate) {
				t.Fatalf("quote=%+v", q)
			}
			if f.fail == "quote" {
				return app.ReservationQuote{}, f.failure
			}
			return app.ReservationQuote{HotelUID: hotelID, StartDate: startDate, EndDate: endDate, Price: 28500}, nil
		},
		create: func(_ context.Context, c app.SaveReservationCommand) error {
			if !f.paid {
				t.Fatal("reservation saved before successful payment")
			}
			if c.ReservationUID != reservationID || c.PaymentUID != paymentID || c.Username != username || c.HotelUID != hotelID || !c.StartDate.Equal(startDate) || !c.EndDate.Equal(endDate) {
				t.Fatalf("save=%+v", c)
			}
			f.saved = true
			return f.writeError("reservation")
		},
		cancel: func(ctx context.Context, u, id string) error {
			if u != username || id != reservationID {
				t.Fatal("incorrect reservation cancellation identity")
			}
			f.checkCleanup(ctx, "reservation")
			f.saved = false
			return f.cleanupFailure
		},
		get: func(_ context.Context, u, id string) (app.ReservationData, error) {
			if u != username || id != reservationID {
				t.Fatal("incorrect ownership query")
			}
			r := reservation()
			if !f.saved {
				r.Status = "CANCELED"
			}
			return r, nil
		},
	}
	f.p = paymentFake{
		create: func(_ context.Context, c app.CreatePaymentCommand) (app.PaymentInfo, error) {
			if f.saved || f.counted {
				t.Fatal("payment must precede reservation and loyalty changes")
			}
			if c.PaymentUID != paymentID || c.Price != 28500 {
				t.Fatalf("payment=%+v", c)
			}
			f.paid = true
			return app.PaymentInfo{Status: "PAID", Price: c.Price}, f.writeError("payment")
		},
		cancel: func(ctx context.Context, id string) error {
			if id != paymentID {
				t.Fatal("incorrect payment cancellation identity")
			}
			f.checkCleanup(ctx, "payment")
			f.paid = false
			return f.cleanupFailure
		},
	}
	f.l = loyaltyFake{
		get: func(_ context.Context, u string) (app.LoyaltyInfo, error) {
			if u != username {
				t.Fatal("incorrect loyalty user")
			}
			if f.fail == "loyalty_get" {
				return app.LoyaltyInfo{}, f.failure
			}
			if f.counted {
				return app.LoyaltyInfo{Status: "SILVER", Discount: 7, ReservationCount: 10}, nil
			}
			return app.LoyaltyInfo{Status: "BRONZE", Discount: 5, ReservationCount: 9}, nil
		},
		add: func(_ context.Context, u, id string) error {
			if !f.paid || !f.saved {
				t.Fatal("loyalty changed before payment and reservation")
			}
			if u != username || id != reservationID {
				t.Fatal("loyalty change is not keyed by booking")
			}
			f.counted = true
			return f.writeError("loyalty")
		},
		remove: func(ctx context.Context, u, id string) error {
			if u != username || id != reservationID {
				t.Fatal("loyalty removal is not keyed by booking")
			}
			f.checkCleanup(ctx, "loyalty")
			f.counted = false
			return f.cleanupFailure
		},
	}
	return f
}

func (f *bookingFixture) checkCleanup(ctx context.Context, operation string) {
	f.t.Helper()
	if ctx.Err() != nil {
		f.t.Fatalf("cleanup uses canceled context: %v", ctx.Err())
	}
	if f.cancelRequest != nil {
		if _, ok := ctx.Deadline(); !ok {
			f.t.Fatal("detached cleanup has no deadline")
		}
	}
	f.cleanup = append(f.cleanup, operation)
}

func (f *bookingFixture) writeError(stage string) error {
	if f.fail != stage {
		return nil
	}
	if f.cancelRequest != nil {
		f.cancelRequest()
	}
	return f.failure
}

func (f *bookingFixture) service() *app.Service {
	return app.NewService(f.r, f.p, f.l, func() (string, error) {
		f.idCalls++
		if f.fail == "id" {
			return "", f.failure
		}
		if f.idCalls == 1 {
			return reservationID, nil
		}
		return paymentID, nil
	}, time.Second)
}

func createCommand() app.CreateReservationCommand {
	return app.CreateReservationCommand{Username: username, HotelUID: hotelID, StartDate: startDate, EndDate: endDate}
}

func TestCreateReservationUsesDiscountBeforeLoyaltyPromotion(t *testing.T) {
	f := newBookingFixture(t)
	got, err := f.service().CreateReservation(context.Background(), createCommand())
	if err != nil {
		t.Fatal(err)
	}
	if got.ReservationUID != reservationID || got.HotelUID != hotelID || got.Status != "PAID" || got.Payment.Status != "PAID" || got.Payment.Price != 28500 || got.Discount != 5 || !got.StartDate.Equal(startDate) || !got.EndDate.Equal(endDate) {
		t.Fatalf("result=%+v", got)
	}
	if !f.paid || !f.saved || !f.counted || len(f.cleanup) != 0 {
		t.Fatalf("incomplete booking: %+v", f)
	}
}

func TestCreateReservationCompensatesCommittedWritesWithLostResponses(t *testing.T) {
	for _, stage := range []string{"loyalty_get", "quote", "id", "payment", "reservation", "loyalty"} {
		t.Run(stage, func(t *testing.T) {
			f := newBookingFixture(t)
			f.fail = stage
			_, err := f.service().CreateReservation(context.Background(), createCommand())
			if !errors.Is(err, f.failure) {
				t.Fatalf("err=%v", err)
			}
			if f.paid || f.saved || f.counted {
				t.Fatalf("left committed effects: paid=%v, saved=%v, counted=%v", f.paid, f.saved, f.counted)
			}
			if (stage == "loyalty_get" || stage == "quote" || stage == "id") && len(f.cleanup) != 0 {
				t.Fatalf("cleanup before any write: %v", f.cleanup)
			}
			if stage == "payment" && !reflect.DeepEqual(f.cleanup, []string{"payment"}) {
				t.Fatalf("cleanup=%v", f.cleanup)
			}
			if stage == "reservation" && !reflect.DeepEqual(f.cleanup, []string{"reservation", "payment"}) {
				t.Fatalf("cleanup=%v", f.cleanup)
			}
			if stage == "loyalty" && !reflect.DeepEqual(f.cleanup, []string{"loyalty", "reservation", "payment"}) {
				t.Fatalf("cleanup=%v", f.cleanup)
			}
		})
	}
}

func TestCreateReservationCleanupSurvivesRequestCancellation(t *testing.T) {
	f := newBookingFixture(t)
	f.fail = "loyalty"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.cancelRequest = cancel
	_, err := f.service().CreateReservation(ctx, createCommand())
	if !errors.Is(err, f.failure) || ctx.Err() == nil || f.paid || f.saved || f.counted {
		t.Fatalf("err=%v, incomplete cleanup=%+v", err, f)
	}
}

func TestCreateReservationAttemptsAllCompensationsAndKeepsTheirErrors(t *testing.T) {
	f := newBookingFixture(t)
	f.fail = "loyalty"
	f.cleanupFailure = errors.New("cleanup unavailable")
	_, err := f.service().CreateReservation(context.Background(), createCommand())
	if !errors.Is(err, f.failure) || !errors.Is(err, f.cleanupFailure) || len(f.cleanup) != 3 {
		t.Fatalf("err=%v, attempted=%v", err, f.cleanup)
	}
}

func TestCreateReservationRejectsInvalidDatesBeforeCallingServices(t *testing.T) {
	for _, end := range []time.Time{startDate, startDate.AddDate(0, 0, -1), {}} {
		cmd := createCommand()
		cmd.EndDate = end
		_, err := app.NewService(reservationFake{}, paymentFake{}, loyaltyFake{}, nil, time.Second).CreateReservation(context.Background(), cmd)
		var validation *app.ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("end=%v, err=%v", end, err)
		}
	}
}

func TestCancelReservationIsSafeToRepeat(t *testing.T) {
	f := newBookingFixture(t)
	f.paid = true
	f.saved = true
	f.counted = true
	s := newService(f.r, f.p, f.l)
	for range 2 {
		if err := s.CancelReservation(context.Background(), username, reservationID); err != nil {
			t.Fatal(err)
		}
		if f.paid || f.saved || f.counted {
			t.Fatal("cancellation left an active effect")
		}
	}
	if len(f.cleanup) != 6 {
		t.Fatalf("repeat did not reconcile all three services: %v", f.cleanup)
	}
}

func TestCancelAlreadyCanceledReservationReconcilesRemainingEffects(t *testing.T) {
	f := newBookingFixture(t)
	f.saved = false
	f.paid = true
	f.counted = true
	if err := newService(f.r, f.p, f.l).CancelReservation(context.Background(), username, reservationID); err != nil {
		t.Fatal(err)
	}
	if f.paid || f.counted || len(f.cleanup) != 3 {
		t.Fatalf("canceled booking still has effects: paid=%v, counted=%v, calls=%v", f.paid, f.counted, f.cleanup)
	}
}

func TestCancelReservationRejectsForeignBookingBeforeAnyWrites(t *testing.T) {
	r := reservationFake{get: func(context.Context, string, string) (app.ReservationData, error) {
		row := reservation()
		row.Username = "Another user"
		return row, nil
	}}
	err := newService(r, paymentFake{}, loyaltyFake{}).CancelReservation(context.Background(), username, reservationID)
	if !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestCancelReservationCanResumeAfterEachPartialFailure(t *testing.T) {
	for _, stage := range []string{"payment", "loyalty", "reservation"} {
		t.Run(stage, func(t *testing.T) {
			f := newBookingFixture(t)
			f.paid = true
			f.saved = true
			f.counted = true
			failure := errors.New("temporary cancellation failure")
			switch stage {
			case "payment":
				original := f.p.cancel
				f.p.cancel = func(ctx context.Context, id string) error { _ = original(ctx, id); return failure }
			case "loyalty":
				original := f.l.remove
				f.l.remove = func(ctx context.Context, u, id string) error { _ = original(ctx, u, id); return failure }
			case "reservation":
				original := f.r.cancel
				f.r.cancel = func(ctx context.Context, u, id string) error { _ = original(ctx, u, id); return failure }
			}
			s := newService(f.r, f.p, f.l)
			if err := s.CancelReservation(context.Background(), username, reservationID); !errors.Is(err, failure) {
				t.Fatalf("first error=%v", err)
			}
			// A new request uses the same booking identity, never another decrement.
			retry := newBookingFixture(t)
			retry.paid = f.paid
			retry.saved = f.saved
			retry.counted = f.counted
			if err := newService(retry.r, retry.p, retry.l).CancelReservation(context.Background(), username, reservationID); err != nil {
				t.Fatal(err)
			}
			if retry.paid || retry.saved || retry.counted {
				t.Fatal("retry did not finish cancellation")
			}
		})
	}
}
