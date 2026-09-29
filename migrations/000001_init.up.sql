CREATE TABLE IF NOT EXISTS users (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login         text        NOT NULL UNIQUE,
    password_hash text        NOT NULL,
    balance       numeric     NOT NULL DEFAULT 0 CHECK (balance >= 0),
    withdrawn     numeric     NOT NULL DEFAULT 0 CHECK (withdrawn >= 0),
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    -- Order numbers are unique across all users and never repeat.
    number      text        PRIMARY KEY,
    user_id     bigint      NOT NULL REFERENCES users (id),
    status      text        NOT NULL DEFAULT 'NEW'
                CHECK (status IN ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED')),
    accrual     numeric     CHECK (accrual >= 0),
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    -- When the accrual system was last asked about the order: the worker
    -- takes the orders it has not asked about for the longest time first.
    checked_at  timestamptz NOT NULL DEFAULT '-infinity'
);

-- The order list of a user, newest first.
CREATE INDEX IF NOT EXISTS orders_user_id_uploaded_at_idx ON orders (user_id, uploaded_at DESC);

-- The queue of the worker: only the orders still waiting for a final status.
CREATE INDEX IF NOT EXISTS orders_pending_idx ON orders (checked_at)
    WHERE status IN ('NEW', 'PROCESSING');

CREATE TABLE IF NOT EXISTS withdrawals (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      bigint      NOT NULL REFERENCES users (id),
    -- An order can be paid with points only once.
    order_number text        NOT NULL UNIQUE,
    sum          numeric     NOT NULL CHECK (sum > 0),
    processed_at timestamptz NOT NULL DEFAULT now()
);

-- The withdrawal list of a user, newest first.
CREATE INDEX IF NOT EXISTS withdrawals_user_id_processed_at_idx ON withdrawals (user_id, processed_at DESC);
