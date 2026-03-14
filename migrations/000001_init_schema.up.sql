CREATE TABLE IF NOT EXISTS processed_messages (
    idempotency_key VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS transactions (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    amount DECIMAL(15, 4) NOT NULL,
    type VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,

    CONSTRAINT chk_transaction_type CHECK (type IN ('bet', 'win')),
    CONSTRAINT chk_transaction_amount CHECK (amount > 0)
);

CREATE INDEX idx_transactions_type_ts_id
    ON transactions (type, timestamp DESC, id DESC);

CREATE INDEX idx_transactions_user_type_ts_id
    ON transactions (user_id, type, timestamp DESC, id DESC);
