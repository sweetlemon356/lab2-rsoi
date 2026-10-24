-- +goose Up
CREATE TABLE loyalty (
    id SERIAL PRIMARY KEY,
    username VARCHAR(80) NOT NULL UNIQUE,
    reservation_count INTEGER NOT NULL DEFAULT 0 CHECK (reservation_count >= 0),
    status VARCHAR(80) NOT NULL DEFAULT 'BRONZE' CHECK (status IN ('BRONZE', 'SILVER', 'GOLD')),
    discount INTEGER NOT NULL DEFAULT 5 CHECK (discount BETWEEN 0 AND 100)
);
CREATE TABLE loyalty_reservations (
    reservation_uid UUID PRIMARY KEY,
    username VARCHAR(80) NOT NULL REFERENCES loyalty(username),
    status VARCHAR(20) NOT NULL CHECK (status IN ('ACTIVE', 'CANCELED'))
);
CREATE INDEX loyalty_reservations_username_idx ON loyalty_reservations(username);

-- +goose Down
DROP TABLE loyalty_reservations;
DROP TABLE loyalty;
