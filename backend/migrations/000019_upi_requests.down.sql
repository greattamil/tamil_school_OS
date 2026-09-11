ALTER TABLE fee_settings DROP COLUMN IF EXISTS upi_payee_name;
ALTER TABLE fee_settings DROP COLUMN IF EXISTS upi_vpa;
DROP TABLE IF EXISTS upi_payment_request_allocations;
DROP TABLE IF EXISTS upi_payment_requests;
DROP TYPE IF EXISTS upi_request_status;
