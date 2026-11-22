-- Deleting cotinhas, hiding them from statistics, and emailing notifications.

-- A deleted cotinha can't be referenced, so its notification keeps the title.
ALTER TABLE notifications ADD COLUMN title text;
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
    'friend_request', 'friend_accepted', 'cotinha_added', 'cotinha_deleted',
    'payment_sent', 'payment_confirmed', 'payment_rejected', 'compensated'));

-- Email delivery of notifications: NULL = waiting for the mail worker.
ALTER TABLE notifications ADD COLUMN email_state text CHECK (email_state IN ('sent', 'skipped', 'failed'));
ALTER TABLE notifications ADD COLUMN emailed_at timestamptz;
-- Nothing that happened before this migration gets emailed.
UPDATE notifications SET email_state = 'skipped';
CREATE INDEX notifications_email_pending_idx ON notifications(created_at) WHERE email_state IS NULL;

ALTER TABLE users ADD COLUMN email_notifications boolean NOT NULL DEFAULT true;

-- Login codes are counted against the same daily email quota.
CREATE TABLE email_log (
    sent_at  timestamptz NOT NULL DEFAULT now(),
    kind     text NOT NULL
);
CREATE INDEX email_log_sent_idx ON email_log(sent_at);

-- Each person decides which cotinhas count in their own statistics.
CREATE TABLE cotinha_stats_hidden (
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cotinha_id  uuid NOT NULL REFERENCES cotinhas(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, cotinha_id)
);
