ALTER TABLE sessions
    ADD COLUMN method text NOT NULL DEFAULT 'password';
ALTER TABLE sessions
    ADD CONSTRAINT sessions_method_check
        CHECK (method = 'password' OR method ~ '^oauth:[a-z0-9-]{1,32}$');
