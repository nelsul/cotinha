package main

import (
	"net/http"
	"sort"
	"time"
)

// countedPayments is every confirmed payment's part in each cotinha ($1 = the
// user, $2 = period start), without compensations and without the cotinhas the
// user hid from their statistics. Use it as "FROM countedPayments p".
const countedPayments = `(
	SELECT p.payer_id, p.payee_id, pa.amount_cents, pa.cotinha_id, p.created_at, p.decided_at
	FROM payments p JOIN payment_allocations pa ON pa.payment_id = p.id
	WHERE p.status = 'confirmed' AND p.method <> 'compensation' AND p.decided_at >= $2
	AND NOT EXISTS (SELECT 1 FROM cotinha_stats_hidden h WHERE h.user_id = $1 AND h.cotinha_id = pa.cotinha_id))`

type StatRow struct {
	User  PublicUser `json:"user"`
	Cents int64      `json:"cents"`
}

type PaySpeed struct {
	User PublicUser `json:"user"`
	Days float64    `json:"days"`
}

// handleStats summarises money moved in a period (month, year or all).
// Compensations are not counted as money sent or received, and cotinhas the
// user hid from their statistics are left out everywhere.
func (a *App) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me := currentUser(ctx)
	now := time.Now()
	since := time.Time{}
	switch r.URL.Query().Get("period") {
	case "month", "":
		since = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "year":
		since = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	}

	var sent, received int64
	var created int
	err := a.db.QueryRow(ctx, `SELECT
		coalesce(sum(amount_cents) FILTER (WHERE payer_id = $1), 0),
		coalesce(sum(amount_cents) FILTER (WHERE payee_id = $1), 0)
		FROM `+countedPayments+` p WHERE payer_id = $1 OR payee_id = $1`, me, since).Scan(&sent, &received)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := a.db.QueryRow(ctx, `SELECT count(*) FROM cotinhas c WHERE creditor_id = $1 AND created_at >= $2
		AND NOT EXISTS (SELECT 1 FROM cotinha_stats_hidden h WHERE h.user_id = $1 AND h.cotinha_id = c.id)`, me, since).Scan(&created); err != nil {
		a.serverError(w, r, err)
		return
	}

	top := func(sql string) ([]StatRow, error) {
		rows, err := a.db.Query(ctx, sql, me, since)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		type raw struct {
			id    string
			cents int64
		}
		var list []raw
		for rows.Next() {
			var x raw
			if err := rows.Scan(&x.id, &x.cents); err != nil {
				return nil, err
			}
			list = append(list, x)
		}
		rows.Close()
		out := []StatRow{}
		for _, x := range list {
			u, err := a.userByID(ctx, x.id)
			if err != nil {
				return nil, err
			}
			out = append(out, StatRow{u, x.cents})
		}
		return out, nil
	}
	sentTo, err := top(`SELECT payee_id, sum(amount_cents) FROM ` + countedPayments + ` p
		WHERE payer_id = $1 GROUP BY payee_id ORDER BY 2 DESC LIMIT 5`)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	paidMe, err := top(`SELECT payer_id, sum(amount_cents) FROM ` + countedPayments + ` p
		WHERE payee_id = $1 GROUP BY payer_id ORDER BY 2 DESC LIMIT 5`)
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	// What friends still owe me, leaving out hidden cotinhas.
	shares, err := sharesInvolving(ctx, a.db, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	hidden := map[string]bool{}
	hrows, err := a.db.Query(ctx, `SELECT cotinha_id FROM cotinha_stats_hidden WHERE user_id = $1`, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	for hrows.Next() {
		var id string
		if err := hrows.Scan(&id); err != nil {
			hrows.Close()
			a.serverError(w, r, err)
			return
		}
		hidden[id] = true
	}
	hrows.Close()
	owed := map[string]int64{}
	for _, sh := range shares {
		if sh.CreditorID == me && !hidden[sh.CotinhaID] && sh.Remaining() > 0 {
			owed[sh.DebtorID] += sh.Remaining()
		}
	}
	owesMe := []StatRow{}
	var toReceive int64
	for id, cents := range owed {
		u, err := a.userByID(ctx, id)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		owesMe = append(owesMe, StatRow{u, cents})
		toReceive += cents
	}
	sort.Slice(owesMe, func(i, j int) bool { return owesMe[i].Cents > owesMe[j].Cents })
	if len(owesMe) > 5 {
		owesMe = owesMe[:5]
	}

	// Average days between a cotinha being created and each friend paying me for it.
	rows, err := a.db.Query(ctx, `
		SELECT p.payer_id, avg(extract(epoch FROM p.created_at - c.created_at) / 86400)
		FROM payments p
		JOIN payment_allocations pa ON pa.payment_id = p.id
		JOIN cotinhas c ON c.id = pa.cotinha_id
		WHERE p.payee_id = $1 AND p.status = 'confirmed' AND p.method <> 'compensation'
		AND NOT EXISTS (SELECT 1 FROM cotinha_stats_hidden h WHERE h.user_id = $1 AND h.cotinha_id = c.id)
		GROUP BY p.payer_id ORDER BY 2`, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	type speedRaw struct {
		id   string
		days float64
	}
	var speeds []speedRaw
	for rows.Next() {
		var s speedRaw
		if err := rows.Scan(&s.id, &s.days); err != nil {
			rows.Close()
			a.serverError(w, r, err)
			return
		}
		speeds = append(speeds, s)
	}
	rows.Close()
	var fastest, slowest *PaySpeed
	if len(speeds) > 0 {
		f, l := speeds[0], speeds[len(speeds)-1]
		fu, err := a.userByID(ctx, f.id)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		fastest = &PaySpeed{fu, f.days}
		if len(speeds) > 1 {
			lu, err := a.userByID(ctx, l.id)
			if err != nil {
				a.serverError(w, r, err)
				return
			}
			slowest = &PaySpeed{lu, l.days}
		}
	}

	writeJSON(w, 200, map[string]any{
		"sent_cents":       sent,
		"received_cents":   received,
		"cotinhas_created": created,
		"to_receive_cents": toReceive,
		"sent_to":          sentTo,
		"owes_me":          owesMe,
		"paid_me":          paidMe,
		"fastest_payer":    fastest,
		"slowest_payer":    slowest,
	})
}
