-- +goose Up
INSERT INTO loyalty(username, reservation_count, status, discount)
VALUES ('Test Max', 25, 'GOLD', 10);

-- +goose Down
DELETE FROM loyalty WHERE username = 'Test Max';
