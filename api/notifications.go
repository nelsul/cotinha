package main

import (
	"context"
	"net/http"
	"time"
)

// notify stores a notification for user. Pass empty strings for the ids that
// don't apply. It runs inside the caller's transaction when given one.
func notify(ctx context.Context, q querier, user, kind, actor, cotinha, payment string, amount int64) error {
	return notifyTitled(ctx, q, user, kind, actor, cotinha, payment, amount, "")
}

// notifyTitled also stores a title, for events whose cotinha no longer exists.
func notifyTitled(ctx context.Context, q querier, user, kind, actor, cotinha, payment string, amount int64, title string) error {
	null := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	var cents *int64
	if amount > 0 {
		cents = &amount
	}
	_, err := q.Exec(ctx, `INSERT INTO notifications (user_id, kind, actor_id, cotinha_id, payment_id, amount_cents, title)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, user, kind, null(actor), null(cotinha), null(payment), cents, null(title))
	return err
}

type NotificationOut struct {
	ID           string      `json:"id"`
	Kind         string      `json:"kind"`
	Actor        *PublicUser `json:"actor"`
	CotinhaID    string      `json:"cotinha_id"`
	CotinhaTitle string      `json:"cotinha_title"`
	PaymentID    string      `json:"payment_id"`
	Amount       int64       `json:"amount_cents"`
	Read         bool        `json:"read"`
	CreatedAt    time.Time   `json:"created_at"`
}

func (a *App) unreadCount(ctx context.Context, user string) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, user).Scan(&n)
	return n, err
}

func (a *App) handleUnreadCount(w http.ResponseWriter, r *http.Request) {
	n, err := a.unreadCount(r.Context(), currentUser(r.Context()))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]int{"unread": n})
}

func (a *App) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me := currentUser(ctx)
	rows, err := a.db.Query(ctx, `
		SELECT n.id, n.kind, n.actor_id, n.cotinha_id, coalesce(c.title, n.title, ''), n.payment_id,
			coalesce(n.amount_cents, 0), n.read_at IS NOT NULL, n.created_at
		FROM notifications n LEFT JOIN cotinhas c ON c.id = n.cotinha_id
		WHERE n.user_id = $1 ORDER BY n.created_at DESC LIMIT 60`, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []NotificationOut{}
	var actors []*string
	for rows.Next() {
		var n NotificationOut
		var actor, cotinha, payment *string
		if err := rows.Scan(&n.ID, &n.Kind, &actor, &cotinha, &n.CotinhaTitle, &payment, &n.Amount, &n.Read, &n.CreatedAt); err != nil {
			a.serverError(w, r, err)
			return
		}
		n.CotinhaID, n.PaymentID = deref(cotinha), deref(payment)
		out = append(out, n)
		actors = append(actors, actor)
	}
	rows.Close()
	users := map[string]PublicUser{}
	for i, id := range actors {
		if id == nil {
			continue
		}
		u, ok := users[*id]
		if !ok {
			if u, err = a.userByID(ctx, *id); err != nil {
				a.serverError(w, r, err)
				return
			}
			users[*id] = u
		}
		out[i].Actor = &u
	}
	writeJSON(w, 200, out)
}

func (a *App) handleReadNotifications(w http.ResponseWriter, r *http.Request) {
	if _, err := a.db.Exec(r.Context(), `UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, currentUser(r.Context())); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
