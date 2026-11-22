package main

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const requestBlockTTL = 24 * time.Hour

type FriendOut struct {
	PublicUser
	NetCents int64 `json:"net_cents"` // > 0: they owe me; < 0: I owe them
}

func (a *App) handleListFriends(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me := currentUser(ctx)
	rows, err := a.db.Query(ctx, `
		SELECT u.id, u.name, u.handle, u.avatar_key
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.user_a = $1 THEN f.user_b ELSE f.user_a END
		WHERE f.user_a = $1 OR f.user_b = $1
		ORDER BY lower(u.name)`, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []FriendOut{}
	for rows.Next() {
		var id, name string
		var handle, avatar *string
		if err := rows.Scan(&id, &name, &handle, &avatar); err != nil {
			a.serverError(w, r, err)
			return
		}
		out = append(out, FriendOut{PublicUser: a.publicUser(ctx, id, name, handle, avatar)})
	}
	rows.Close()
	bal, err := a.pairBalances(ctx, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	for i := range out {
		if b, ok := bal[out[i].ID]; ok {
			out[i].NetCents = b.TheyOwe - b.IOwe
		}
	}
	writeJSON(w, 200, out)
}

func (a *App) handleRemoveFriend(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	bal, err := a.pairBalances(ctx, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if b, ok := bal[id]; ok && (b.TheyOwe > 0 || b.IOwe > 0) {
		writeError(w, 409, "Acertem as contas antes de desfazer a amizade.")
		return
	}
	u1, u2 := orderedPair(me, id)
	if _, err := a.db.Exec(ctx, `DELETE FROM friendships WHERE user_a = $1 AND user_b = $2`, u1, u2); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type RequestOut struct {
	ID        string     `json:"id"`
	User      PublicUser `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
}

type BlockOut struct {
	User  PublicUser `json:"user"`
	Until time.Time  `json:"until"`
}

func (a *App) handleListRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me := currentUser(ctx)
	load := func(q string) ([]RequestOut, error) {
		rows, err := a.db.Query(ctx, q, me)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []RequestOut{}
		for rows.Next() {
			var rq RequestOut
			var uid, name string
			var handle, avatar *string
			if err := rows.Scan(&rq.ID, &rq.CreatedAt, &uid, &name, &handle, &avatar); err != nil {
				return nil, err
			}
			rq.User = a.publicUser(ctx, uid, name, handle, avatar)
			out = append(out, rq)
		}
		return out, rows.Err()
	}
	incoming, err := load(`SELECT fr.id, fr.created_at, u.id, u.name, u.handle, u.avatar_key
		FROM friend_requests fr JOIN users u ON u.id = fr.from_id
		WHERE fr.to_id = $1 AND fr.status = 'pending' ORDER BY fr.created_at DESC`)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	outgoing, err := load(`SELECT fr.id, fr.created_at, u.id, u.name, u.handle, u.avatar_key
		FROM friend_requests fr JOIN users u ON u.id = fr.to_id
		WHERE fr.from_id = $1 AND fr.status = 'pending' ORDER BY fr.created_at DESC`)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	rows, err := a.db.Query(ctx, `SELECT b.until, u.id, u.name, u.handle, u.avatar_key
		FROM request_blocks b JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id = $1 AND b.until > now() ORDER BY b.until`, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer rows.Close()
	blocks := []BlockOut{}
	for rows.Next() {
		var b BlockOut
		var uid, name string
		var handle, avatar *string
		if err := rows.Scan(&b.Until, &uid, &name, &handle, &avatar); err != nil {
			a.serverError(w, r, err)
			return
		}
		b.User = a.publicUser(ctx, uid, name, handle, avatar)
		blocks = append(blocks, b)
	}
	writeJSON(w, 200, map[string]any{"incoming": incoming, "outgoing": outgoing, "blocks": blocks})
}

func (a *App) handleSendRequest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Handle string `json:"handle"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	h := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(in.Handle), "@"))
	var to string
	err := a.db.QueryRow(ctx, `SELECT id FROM users WHERE handle = $1`, h).Scan(&to)
	if isNoRows(err) {
		writeError(w, 404, "Ninguém usa esse @.")
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	if to == me {
		writeError(w, 400, "Esse é você!")
		return
	}
	rel, err := a.relation(ctx, me, to)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	switch rel {
	case "friend":
		writeError(w, 409, "Vocês já são amigos.")
		return
	case "request_sent":
		writeJSON(w, 200, map[string]string{"relation": "request_sent"})
		return
	case "request_received":
		// They already asked: sending one back means accepting.
		var id string
		if err := a.db.QueryRow(ctx, `SELECT id FROM friend_requests WHERE from_id = $1 AND to_id = $2 AND status = 'pending'`, to, me).Scan(&id); err != nil {
			a.serverError(w, r, err)
			return
		}
		if err := a.acceptRequest(r, id, me, to); err != nil {
			a.serverError(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]string{"relation": "friend"})
		return
	}
	var blocked bool
	if err := a.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM request_blocks WHERE blocker_id = $1 AND blocked_id = $2 AND until > now())`, to, me).Scan(&blocked); err != nil {
		a.serverError(w, r, err)
		return
	}
	if blocked {
		writeError(w, 429, "Essa pessoa não está recebendo pedidos seus agora. Tente amanhã.")
		return
	}
	tag, err := a.db.Exec(ctx, `INSERT INTO friend_requests (from_id, to_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, me, to)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if tag.RowsAffected() > 0 {
		if err := notify(ctx, a.db, to, "friend_request", me, "", "", 0); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	writeJSON(w, 200, map[string]string{"relation": "request_sent"})
}

func (a *App) acceptRequest(r *http.Request, id, me, from string) error {
	ctx := r.Context()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE friend_requests SET status = 'accepted', responded_at = now() WHERE id = $1`, id); err != nil {
		return err
	}
	// Close a crossed request in the other direction too.
	if _, err := tx.Exec(ctx, `UPDATE friend_requests SET status = 'accepted', responded_at = now()
		WHERE from_id = $1 AND to_id = $2 AND status = 'pending'`, me, from); err != nil {
		return err
	}
	u1, u2 := orderedPair(me, from)
	if _, err := tx.Exec(ctx, `INSERT INTO friendships (user_a, user_b) VALUES ($1, $2) ON CONFLICT DO NOTHING`, u1, u2); err != nil {
		return err
	}
	if err := notify(ctx, tx, from, "friend_accepted", me, "", "", 0); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *App) handleRequestAction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	action := chi.URLParam(r, "action")
	ctx := r.Context()
	me := currentUser(ctx)
	var from, to string
	err := a.db.QueryRow(ctx, `SELECT from_id, to_id FROM friend_requests WHERE id = $1 AND status = 'pending'`, id).Scan(&from, &to)
	if isNoRows(err) {
		writeError(w, 404, "Esse pedido não existe mais.")
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	switch {
	case action == "cancel" && from == me:
		_, err = a.db.Exec(ctx, `UPDATE friend_requests SET status = 'cancelled', responded_at = now() WHERE id = $1`, id)
	case action == "accept" && to == me:
		err = a.acceptRequest(r, id, me, from)
	case action == "decline" && to == me:
		_, err = a.db.Exec(ctx, `UPDATE friend_requests SET status = 'declined', responded_at = now() WHERE id = $1`, id)
	case action == "block" && to == me:
		_, err = a.db.Exec(ctx, `UPDATE friend_requests SET status = 'declined', responded_at = now() WHERE id = $1`, id)
		if err == nil {
			_, err = a.db.Exec(ctx, `INSERT INTO request_blocks (blocker_id, blocked_id, until) VALUES ($1, $2, $3)
				ON CONFLICT (blocker_id, blocked_id) DO UPDATE SET until = EXCLUDED.until`, me, from, time.Now().Add(requestBlockTTL))
		}
	default:
		writeError(w, 404, "Não encontrado.")
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *App) handleUnblock(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM request_blocks WHERE blocker_id = $1 AND blocked_id = $2`, currentUser(r.Context()), id); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// sortByAbsNet orders balances with the biggest amounts first.
func sortByAbsNet(list []BalanceOut) {
	sort.SliceStable(list, func(i, j int) bool {
		ai, aj := list[i].TheyOwe+list[i].IOwe, list[j].TheyOwe+list[j].IOwe
		return ai > aj
	})
}
