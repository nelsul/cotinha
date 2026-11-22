package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Share is one debtor's part of a cotinha, with what has been paid so far.
type Share struct {
	CotinhaID  string    `json:"cotinha_id"`
	Title      string    `json:"title"`
	CreatedAt  time.Time `json:"created_at"`
	CreditorID string    `json:"creditor_id"`
	DebtorID   string    `json:"debtor_id"`
	Amount     int64     `json:"amount_cents"`
	Confirmed  int64     `json:"confirmed_cents"`
	Pending    int64     `json:"pending_cents"`
}

// Remaining is what is still owed once confirmed payments are counted.
func (s Share) Remaining() int64 { return s.Amount - s.Confirmed }

// Payable is what can still be paid: pending payments are already on their way.
func (s Share) Payable() int64 { return s.Amount - s.Confirmed - s.Pending }

const sharesSQL = `
	WITH paid AS (
		SELECT pa.cotinha_id, p.payer_id,
			sum(pa.amount_cents) FILTER (WHERE p.status = 'confirmed') AS confirmed,
			sum(pa.amount_cents) FILTER (WHERE p.status = 'pending') AS pending
		FROM payment_allocations pa JOIN payments p ON p.id = pa.payment_id
		WHERE p.status IN ('pending', 'confirmed')
		GROUP BY pa.cotinha_id, p.payer_id
	)
	SELECT c.id, c.title, c.created_at, c.creditor_id, s.user_id, s.amount_cents,
		coalesce(paid.confirmed, 0), coalesce(paid.pending, 0)
	FROM cotinha_shares s
	JOIN cotinhas c ON c.id = s.cotinha_id
	LEFT JOIN paid ON paid.cotinha_id = s.cotinha_id AND paid.payer_id = s.user_id
	WHERE s.user_id <> c.creditor_id AND s.amount_cents > 0`

func scanShares(rows pgx.Rows) ([]Share, error) {
	defer rows.Close()
	var out []Share
	for rows.Next() {
		var s Share
		if err := rows.Scan(&s.CotinhaID, &s.Title, &s.CreatedAt, &s.CreditorID, &s.DebtorID, &s.Amount, &s.Confirmed, &s.Pending); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// sharesInvolving returns every debtor share where the user is creditor or debtor.
func sharesInvolving(ctx context.Context, q querier, user string) ([]Share, error) {
	rows, err := q.Query(ctx, sharesSQL+` AND (c.creditor_id = $1 OR s.user_id = $1) ORDER BY c.created_at`, user)
	if err != nil {
		return nil, err
	}
	return scanShares(rows)
}

// sharesOwed returns debtor's shares in creditor's cotinhas, oldest first.
func sharesOwed(ctx context.Context, q querier, debtor, creditor string) ([]Share, error) {
	rows, err := q.Query(ctx, sharesSQL+` AND s.user_id = $1 AND c.creditor_id = $2 ORDER BY c.created_at, c.id`, debtor, creditor)
	if err != nil {
		return nil, err
	}
	return scanShares(rows)
}

func sharesOfCotinha(ctx context.Context, q querier, cotinha string) ([]Share, error) {
	rows, err := q.Query(ctx, sharesSQL+` AND c.id = $1`, cotinha)
	if err != nil {
		return nil, err
	}
	return scanShares(rows)
}

// PairBalance is what is still open between the current user and one other person.
type PairBalance struct {
	TheyOwe    int64 `json:"they_owe_cents"` // remaining, confirmed payments counted
	IOwe       int64 `json:"i_owe_cents"`
	PendingIn  int64 `json:"pending_in_cents"`  // they said they paid, waiting for me
	PendingOut int64 `json:"pending_out_cents"` // I said I paid, waiting for them
}

func (a *App) pairBalances(ctx context.Context, me string) (map[string]PairBalance, error) {
	shares, err := sharesInvolving(ctx, a.db, me)
	if err != nil {
		return nil, err
	}
	out := map[string]PairBalance{}
	for _, s := range shares {
		if s.CreditorID == me {
			b := out[s.DebtorID]
			b.TheyOwe += s.Remaining()
			b.PendingIn += s.Pending
			out[s.DebtorID] = b
		} else {
			b := out[s.CreditorID]
			b.IOwe += s.Remaining()
			b.PendingOut += s.Pending
			out[s.CreditorID] = b
		}
	}
	return out, nil
}

// lockPair serialises money changes between two people inside a transaction.
func lockPair(ctx context.Context, tx pgx.Tx, x, y string) error {
	u1, u2 := orderedPair(x, y)
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, u1+":"+u2)
	return err
}

type allocation struct {
	CotinhaID string
	Amount    int64
}

// allocate spreads amount over the shares' payable parts, oldest first.
// It returns nil when the shares cannot absorb the whole amount.
func allocate(shares []Share, amount int64) []allocation {
	var out []allocation
	left := amount
	for _, s := range shares {
		if left == 0 {
			break
		}
		p := s.Payable()
		if p <= 0 {
			continue
		}
		take := min(p, left)
		out = append(out, allocation{s.CotinhaID, take})
		left -= take
	}
	if left > 0 {
		return nil
	}
	return out
}

func sumPayable(shares []Share) int64 {
	var t int64
	for _, s := range shares {
		if p := s.Payable(); p > 0 {
			t += p
		}
	}
	return t
}

// insertPayment stores a payment and its allocations.
func insertPayment(ctx context.Context, tx pgx.Tx, payer, payee, method, status string, confirmedBy *string, receipt *string, allocs []allocation) (string, error) {
	var total int64
	for _, al := range allocs {
		total += al.Amount
	}
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO payments (payer_id, payee_id, amount_cents, method, status, confirmed_by, receipt_key, decided_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CASE WHEN $5 = 'confirmed' THEN now() END)
		RETURNING id`, payer, payee, total, method, status, confirmedBy, receipt).Scan(&id)
	if err != nil {
		return "", err
	}
	for _, al := range allocs {
		if _, err := tx.Exec(ctx, `INSERT INTO payment_allocations (payment_id, cotinha_id, amount_cents) VALUES ($1, $2, $3)`, id, al.CotinhaID, al.Amount); err != nil {
			return "", err
		}
	}
	return id, nil
}
