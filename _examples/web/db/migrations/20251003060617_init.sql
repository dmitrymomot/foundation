-- +goose Up
-- +goose StatementBegin

-- Enable required extensions for UUID generation and cryptographic functions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create a function to automatically update the updated_at column
CREATE OR REPLACE FUNCTION update_updated_at_column()
    RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE 'plpgsql';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop extensions (commented out by default as they might be used by other tables)
-- DROP EXTENSION IF EXISTS "uuid-ossp";

-- +goose StatementEnd
