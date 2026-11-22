-- Cotinha initial schema. Money is always stored as integer cents.

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         citext NOT NULL UNIQUE,
    name          text NOT NULL DEFAULT '',
    handle        citext UNIQUE CHECK (handle ~ '^[a-z0-9._]{3,20}$'),
    avatar_key    text,
    pix_key_type  text CHECK (pix_key_type IN ('cpf', 'cnpj', 'email', 'phone', 'random')),
    pix_key       text,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE login_codes (
    email       citext PRIMARY KEY,
    code_hash   text NOT NULL,
    attempts    int NOT NULL DEFAULT 0,
    sent_at     timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL
);

CREATE TABLE sessions (
    token_hash  text PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions(user_id);

-- Friendships are stored once per pair, with user_a < user_b.
CREATE TABLE friendships (
    user_a      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_a, user_b),
    CHECK (user_a < user_b)
);
CREATE INDEX friendships_b_idx ON friendships(user_b);

CREATE TABLE friend_requests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    from_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status        text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    responded_at  timestamptz,
    CHECK (from_id <> to_id)
);
CREATE UNIQUE INDEX friend_requests_pending_idx ON friend_requests(from_id, to_id) WHERE status = 'pending';
CREATE INDEX friend_requests_to_idx ON friend_requests(to_id, status);

-- "Block requests from this person for a day".
CREATE TABLE request_blocks (
    blocker_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    until       timestamptz NOT NULL,
    PRIMARY KEY (blocker_id, blocked_id)
);

CREATE TABLE cotinhas (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    creditor_id  uuid NOT NULL REFERENCES users(id),
    title        text NOT NULL CHECK (length(title) BETWEEN 1 AND 80),
    description  text NOT NULL DEFAULT '' CHECK (length(description) <= 1000),
    total_cents  bigint NOT NULL CHECK (total_cents > 0),
    split_mode   text NOT NULL CHECK (split_mode IN ('equal', 'amount', 'weight')),
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX cotinhas_creditor_idx ON cotinhas(creditor_id);

-- Every participant's part, the creditor's own part included (it is never owed).
CREATE TABLE cotinha_shares (
    cotinha_id    uuid NOT NULL REFERENCES cotinhas(id) ON DELETE CASCADE,
    user_id       uuid NOT NULL REFERENCES users(id),
    amount_cents  bigint NOT NULL CHECK (amount_cents >= 0),
    weight        numeric,
    PRIMARY KEY (cotinha_id, user_id)
);
CREATE INDEX cotinha_shares_user_idx ON cotinha_shares(user_id);

CREATE TABLE cotinha_photos (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cotinha_id   uuid NOT NULL REFERENCES cotinhas(id) ON DELETE CASCADE,
    object_key   text NOT NULL,
    uploaded_by  uuid NOT NULL REFERENCES users(id),
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX cotinha_photos_cotinha_idx ON cotinha_photos(cotinha_id);

-- A payment from one person to another. It is split across that pair's open
-- shares by payment_allocations. The method/provider columns are ready for
-- automatic Pix and cards; the MVP only creates pix_manual and compensation.
CREATE TABLE payments (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    payer_id      uuid NOT NULL REFERENCES users(id),
    payee_id      uuid NOT NULL REFERENCES users(id),
    amount_cents  bigint NOT NULL CHECK (amount_cents > 0),
    fee_cents     bigint NOT NULL DEFAULT 0 CHECK (fee_cents >= 0),
    method        text NOT NULL
                  CHECK (method IN ('pix_manual', 'pix_auto', 'debit', 'credit', 'apple_pay', 'compensation')),
    status        text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'confirmed', 'rejected')),
    confirmed_by  text CHECK (confirmed_by IN ('user', 'webhook', 'system')),
    receipt_key   text,
    provider      text,
    provider_ref  text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    decided_at    timestamptz,
    CHECK (payer_id <> payee_id)
);
CREATE INDEX payments_payee_idx ON payments(payee_id, status);
CREATE INDEX payments_payer_idx ON payments(payer_id, status);
CREATE UNIQUE INDEX payments_provider_ref_idx ON payments(provider, provider_ref) WHERE provider_ref IS NOT NULL;

CREATE TABLE payment_allocations (
    payment_id    uuid NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    cotinha_id    uuid NOT NULL REFERENCES cotinhas(id) ON DELETE CASCADE,
    amount_cents  bigint NOT NULL CHECK (amount_cents > 0),
    PRIMARY KEY (payment_id, cotinha_id)
);
CREATE INDEX payment_allocations_cotinha_idx ON payment_allocations(cotinha_id);
