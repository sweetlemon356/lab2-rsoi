-- +goose Up
CREATE TABLE payment (
    id SERIAL PRIMARY KEY,
    payment_uid UUID NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL CHECK (status IN ('PAID', 'CANCELED')),
    price BIGINT NOT NULL CHECK (price >= 0)
);
CREATE TABLE payment_cancellations (
    payment_uid UUID PRIMARY KEY
);

-- +goose Down
DROP TABLE payment_cancellations;
DROP TABLE payment;
