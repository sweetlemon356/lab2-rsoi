-- +goose Up
CREATE TABLE hotels (
    id SERIAL PRIMARY KEY,
    hotel_uid UUID NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    country VARCHAR(80) NOT NULL,
    city VARCHAR(80) NOT NULL,
    address VARCHAR(255) NOT NULL,
    stars INTEGER CHECK (stars BETWEEN 0 AND 5),
    price BIGINT NOT NULL CHECK (price >= 0)
);
CREATE TABLE reservation (
    id SERIAL PRIMARY KEY,
    reservation_uid UUID NOT NULL UNIQUE,
    username VARCHAR(80) NOT NULL,
    payment_uid UUID NOT NULL,
    hotel_id INTEGER NOT NULL REFERENCES hotels(id),
    status VARCHAR(20) NOT NULL CHECK (status IN ('PAID', 'CANCELED')),
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    CHECK (end_date > start_date)
);
CREATE INDEX reservation_username_idx ON reservation(username, id);
CREATE TABLE reservation_cancellations (
    reservation_uid UUID PRIMARY KEY,
    username VARCHAR(80) NOT NULL
);

-- +goose Down
DROP TABLE reservation_cancellations;
DROP TABLE reservation;
DROP TABLE hotels;
