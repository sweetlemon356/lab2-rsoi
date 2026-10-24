-- +goose Up
INSERT INTO hotels(hotel_uid, name, country, city, address, stars, price)
VALUES ('049161bb-badd-4fa8-9d90-87c9a82b0668', 'Ararat Park Hyatt Moscow',
        'Россия', 'Москва', 'Неглинная ул., 4', 5, 10000);

-- +goose Down
DELETE FROM hotels WHERE hotel_uid = '049161bb-badd-4fa8-9d90-87c9a82b0668';
