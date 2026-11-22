package main

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

type BalanceOut struct {
	User PublicUser `json:"user"`
	PairBalance
}

func (a *App) userByID(ctx context.Context, id string) (PublicUser, error) {
	var name string
	var handle, avatar *string
	err := a.db.QueryRow(ctx, `SELECT name, handle, avatar_key FROM users WHERE id = $1`, id).Scan(&name, &handle, &avatar)
	return a.publicUser(ctx, id, name, handle, avatar), err
}

// handleBalances lists everyone the user has open money with.
func (a *App) handleBalances(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me := currentUser(ctx)
	bal, err := a.pairBalances(ctx, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	out := []BalanceOut{}
	var toReceive, toPay int64
	for id, b := range bal {
		if b.TheyOwe == 0 && b.IOwe == 0 {
			continue
		}
		u, err := a.userByID(ctx, id)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		out = append(out, BalanceOut{User: u, PairBalance: b})
		toReceive += b.TheyOwe
		toPay += b.IOwe
	}
	sortByAbsNet(out)
	writeJSON(w, 200, map[string]any{"to_receive_cents": toReceive, "to_pay_cents": toPay, "people": out})
}

type ShareOut struct {
	CotinhaID string    `json:"cotinha_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	Amount    int64     `json:"amount_cents"`
	Remaining int64     `json:"remaining_cents"`
	Payable   int64     `json:"payable_cents"`
	Pending   int64     `json:"pending_cents"`
}

func toShareOut(s Share) ShareOut {
	return ShareOut{s.CotinhaID, s.Title, s.CreatedAt, s.Amount, s.Remaining(), s.Payable(), s.Pending}
}

// handleBalanceWith shows everything open with one person, for the pay screen.
func (a *App) handleBalanceWith(w http.ResponseWriter, r *http.Request) {
	other, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	user, err := a.userByID(ctx, other)
	if isNoRows(err) {
		writeError(w, 404, "Não encontrado.")
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	mine, err := sharesOwed(ctx, a.db, me, other)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	theirs, err := sharesOwed(ctx, a.db, other, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	iOwe, theyOwe := []ShareOut{}, []ShareOut{}
	var iOweTotal, theyOweTotal int64
	for _, s := range mine {
		if s.Remaining() > 0 {
			iOwe = append(iOwe, toShareOut(s))
			iOweTotal += s.Remaining()
		}
	}
	for _, s := range theirs {
		if s.Remaining() > 0 {
			theyOwe = append(theyOwe, toShareOut(s))
			theyOweTotal += s.Remaining()
		}
	}
	writeJSON(w, 200, map[string]any{
		"user":                 user,
		"i_owe_cents":          iOweTotal,
		"they_owe_cents":       theyOweTotal,
		"i_can_pay_cents":      sumPayable(mine),
		"can_compensate_cents": min(sumPayable(mine), sumPayable(theirs)),
		"i_owe":                iOwe,
		"they_owe":             theyOwe,
	})
}

// handleCompensate cancels out mutual debts: if I owe 60 and they owe 40,
// both 40s are settled at once and only 20 is left for me to pay.
func (a *App) handleCompensate(w http.ResponseWriter, r *http.Request) {
	other, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	tx, err := a.db.Begin(ctx)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer tx.Rollback(ctx)
	if err := lockPair(ctx, tx, me, other); err != nil {
		a.serverError(w, r, err)
		return
	}
	mine, err := sharesOwed(ctx, tx, me, other)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	theirs, err := sharesOwed(ctx, tx, other, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	amount := min(sumPayable(mine), sumPayable(theirs))
	if amount == 0 {
		writeError(w, 409, "Não há dívidas dos dois lados para compensar.")
		return
	}
	system := "system"
	if _, err := insertPayment(ctx, tx, me, other, "compensation", "confirmed", &system, nil, allocate(mine, amount)); err != nil {
		a.serverError(w, r, err)
		return
	}
	if _, err := insertPayment(ctx, tx, other, me, "compensation", "confirmed", &system, nil, allocate(theirs, amount)); err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := notify(ctx, tx, other, "compensated", me, "", "", amount); err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]int64{"compensated_cents": amount})
}

// handleCreatePayment records a manual Pix: the payer says they paid and may
// attach a receipt; the payee confirms later. Multipart fields: payee_id,
// amount_cents, optional cotinha_id, optional file (receipt).
func (a *App) handleCreatePayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me := currentUser(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+1<<20)
	if err := r.ParseMultipartForm(maxImageBytes + 1<<20); err != nil {
		writeError(w, 400, "Requisição inválida.")
		return
	}
	payee := r.FormValue("payee_id")
	cotinha := r.FormValue("cotinha_id")
	amount, err := strconv.ParseInt(r.FormValue("amount_cents"), 10, 64)
	if !uuidRe.MatchString(payee) || (cotinha != "" && !uuidRe.MatchString(cotinha)) || err != nil || amount <= 0 {
		writeError(w, 400, "Requisição inválida.")
		return
	}
	var receipt *string
	if _, _, err := r.FormFile("file"); err == nil {
		data, ctype, ext, ok := readImage(w, r, photoSide)
		if !ok {
			return
		}
		key, err := a.store.Put(ctx, "receipts/"+me, ext, ctype, data)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		receipt = &key
	}
	fail := func(status int, msg string) {
		if receipt != nil {
			a.store.Delete(context.Background(), *receipt)
		}
		writeError(w, status, msg)
	}

	tx, err := a.db.Begin(ctx)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer tx.Rollback(ctx)
	if err := lockPair(ctx, tx, me, payee); err != nil {
		a.serverError(w, r, err)
		return
	}
	shares, err := sharesOwed(ctx, tx, me, payee)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if cotinha != "" {
		var only []Share
		for _, s := range shares {
			if s.CotinhaID == cotinha {
				only = append(only, s)
			}
		}
		shares = only
	}
	payable := sumPayable(shares)
	if payable == 0 {
		fail(409, "Não há nada em aberto para pagar.")
		return
	}
	if amount > payable {
		fail(400, "O valor é maior do que você deve.")
		return
	}
	allocs := allocate(shares, amount)
	id, err := insertPayment(ctx, tx, me, payee, "pix_manual", "pending", nil, receipt, allocs)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := notify(ctx, tx, payee, "payment_sent", me, allocs[0].CotinhaID, id, amount); err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

type PaymentOut struct {
	ID          string         `json:"id"`
	Payer       PublicUser     `json:"payer"`
	Payee       PublicUser     `json:"payee"`
	Amount      int64          `json:"amount_cents"`
	Fee         int64          `json:"fee_cents"`
	Method      string         `json:"method"`
	Status      string         `json:"status"`
	ConfirmedBy string         `json:"confirmed_by"`
	ReceiptURL  string         `json:"receipt_url"`
	CreatedAt   time.Time      `json:"created_at"`
	DecidedAt   *time.Time     `json:"decided_at"`
	Cotinhas    []PaymentAlloc `json:"cotinhas"`
}

type PaymentAlloc struct {
	CotinhaID string `json:"cotinha_id"`
	Title     string `json:"title"`
	Amount    int64  `json:"amount_cents"`
}

const paymentSelect = `
	SELECT p.id, p.payer_id, p.payee_id, p.amount_cents, p.fee_cents, p.method, p.status,
		coalesce(p.confirmed_by, ''), p.receipt_key, p.created_at, p.decided_at
	FROM payments p`

func (a *App) loadPayments(ctx context.Context, where string, args ...any) ([]PaymentOut, error) {
	rows, err := a.db.Query(ctx, paymentSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PaymentOut
	var payers, payees []string
	for rows.Next() {
		var p PaymentOut
		var payer, payee string
		var receipt *string
		if err := rows.Scan(&p.ID, &payer, &payee, &p.Amount, &p.Fee, &p.Method, &p.Status, &p.ConfirmedBy, &receipt, &p.CreatedAt, &p.DecidedAt); err != nil {
			return nil, err
		}
		p.ReceiptURL = a.store.URL(ctx, receipt)
		out = append(out, p)
		payers = append(payers, payer)
		payees = append(payees, payee)
	}
	rows.Close()
	for i := range out {
		if out[i].Payer, err = a.userByID(ctx, payers[i]); err != nil {
			return nil, err
		}
		if out[i].Payee, err = a.userByID(ctx, payees[i]); err != nil {
			return nil, err
		}
		arows, err := a.db.Query(ctx, `SELECT c.id, c.title, pa.amount_cents FROM payment_allocations pa
			JOIN cotinhas c ON c.id = pa.cotinha_id WHERE pa.payment_id = $1 ORDER BY c.created_at`, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Cotinhas = []PaymentAlloc{}
		for arows.Next() {
			var al PaymentAlloc
			if err := arows.Scan(&al.CotinhaID, &al.Title, &al.Amount); err != nil {
				arows.Close()
				return nil, err
			}
			out[i].Cotinhas = append(out[i].Cotinhas, al)
		}
		arows.Close()
	}
	return out, nil
}

func (a *App) handlePaymentsToConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := a.loadPayments(ctx, `WHERE p.payee_id = $1 AND p.status = 'pending' ORDER BY p.created_at DESC`, currentUser(ctx))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if list == nil {
		list = []PaymentOut{}
	}
	writeJSON(w, 200, list)
}

func (a *App) handleGetPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	list, err := a.loadPayments(ctx, `WHERE p.id = $1 AND (p.payer_id = $2 OR p.payee_id = $2)`, id, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if len(list) == 0 {
		writeError(w, 404, "Pagamento não encontrado.")
		return
	}
	writeJSON(w, 200, list[0])
}

func (a *App) decidePayment(w http.ResponseWriter, r *http.Request, status string) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	tag, err := a.db.Exec(ctx, `UPDATE payments SET status = $1, decided_at = now(),
		confirmed_by = CASE WHEN $1 = 'confirmed' THEN 'user' END
		WHERE id = $2 AND payee_id = $3 AND status = 'pending'`, status, id, currentUser(ctx))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	if tag.RowsAffected() == 0 {
		var mine bool
		_ = a.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM payments WHERE id = $1 AND payee_id = $2)`, id, currentUser(ctx)).Scan(&mine)
		if !mine {
			writeError(w, 403, "Só quem recebe o pagamento pode confirmar ou recusar.")
			return
		}
		writeError(w, 409, "Esse pagamento já foi resolvido.")
		return
	}
	var payer string
	var amount int64
	var cotinha *string
	if err := a.db.QueryRow(ctx, `SELECT p.payer_id, p.amount_cents,
		(SELECT cotinha_id FROM payment_allocations WHERE payment_id = p.id LIMIT 1)
		FROM payments p WHERE p.id = $1`, id).Scan(&payer, &amount, &cotinha); err != nil {
		a.serverError(w, r, err)
		return
	}
	if err := notify(ctx, a.db, payer, "payment_"+status, currentUser(ctx), deref(cotinha), id, amount); err != nil {
		a.serverError(w, r, err)
		return
	}
	a.handleGetPayment(w, r)
}

func (a *App) handleConfirmPayment(w http.ResponseWriter, r *http.Request) {
	a.decidePayment(w, r, "confirmed")
}
func (a *App) handleRejectPayment(w http.ResponseWriter, r *http.Request) {
	a.decidePayment(w, r, "rejected")
}
