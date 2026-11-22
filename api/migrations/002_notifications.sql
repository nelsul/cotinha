-- In-app notifications: friend requests, new cotinhas, payments to confirm and their outcome.
CREATE TABLE notifications (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          text NOT NULL CHECK (kind IN (
                      'friend_request', 'friend_accepted', 'cotinha_added',
                      'payment_sent', 'payment_confirmed', 'payment_rejected', 'compensated')),
    actor_id      uuid REFERENCES users(id) ON DELETE CASCADE,
    cotinha_id    uuid REFERENCES cotinhas(id) ON DELETE CASCADE,
    payment_id    uuid REFERENCES payments(id) ON DELETE CASCADE,
    amount_cents  bigint,
    read_at       timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications(user_id, created_at DESC);
CREATE INDEX notifications_unread_idx ON notifications(user_id) WHERE read_at IS NULL;
