CREATE DATABASE reservation OWNER reservation_service;
CREATE DATABASE payment OWNER payment_service;
CREATE DATABASE loyalty OWNER loyalty_service;

REVOKE CONNECT ON DATABASE reservation, payment, loyalty FROM PUBLIC;
GRANT CONNECT ON DATABASE reservation TO reservation_service;
GRANT CONNECT ON DATABASE payment TO payment_service;
GRANT CONNECT ON DATABASE loyalty TO loyalty_service;
