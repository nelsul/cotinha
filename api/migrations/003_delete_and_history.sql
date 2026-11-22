-- Deleting cotinhas and hiding them from statistics.

-- A deleted cotinha can't be referenced, so its notification keeps the title.
ALTER TABLE notifications ADD COLUMN title text;
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
    'friend_request', 'friend_accepted', 'cotinha_added', 'cotinha_deleted',
    'payment_sent', 'payment_confirmed', 'payment_rejected', 'compensated'));

-- Each person decides which cotinhas count in their own statistics.
CREATE TABLE cotinha_stats_hidden (
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cotinha_id  uuid NOT NULL REFERENCES cotinhas(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, cotinha_id)
);
