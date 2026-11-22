package main

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const maxPhotos = 6

type participantIn struct {
	UserID string  `json:"user_id"`
	Value  float64 `json:"value"` // cents for "amount", weight for "weight", ignored for "equal"
}

// splitShares turns a total into per-person cents. The sum always equals total.
func splitShares(total int64, mode string, parts []participantIn) ([]int64, string) {
	n := len(parts)
	out := make([]int64, n)
	switch mode {
	case "equal":
		base, rem := total/int64(n), total%int64(n)
		for i := range out {
			out[i] = base
			if int64(i) < rem {
				out[i]++
			}
		}
	case "amount":
		var sum int64
		for i, p := range parts {
			if p.Value < 0 || p.Value != math.Trunc(p.Value) {
				return nil, "Os valores precisam ser em centavos inteiros."
			}
			out[i] = int64(p.Value)
			sum += out[i]
		}
		if sum != total {
			return nil, "A soma das partes precisa ser igual ao total."
		}
	case "weight":
		var wsum float64
		for _, p := range parts {
			if p.Value < 0 || p.Value > 1000 {
				return nil, "Os pesos precisam estar entre 0 e 1000."
			}
			wsum += p.Value
		}
		if wsum <= 0 {
			return nil, "Pelo menos uma pessoa precisa ter peso maior que zero."
		}
		type frac struct {
			i int
			f float64
		}
		var given int64
		fr := make([]frac, n)
		for i, p := range parts {
			exact := float64(total) * p.Value / wsum
			out[i] = int64(math.Floor(exact))
			given += out[i]
			fr[i] = frac{i, exact - math.Floor(exact)}
		}
		sort.SliceStable(fr, func(a, b int) bool { return fr[a].f > fr[b].f })
		for k := 0; given < total; k++ {
			out[fr[k%n].i]++
			given++
		}
	default:
		return nil, "Modo de divisão inválido."
	}
	return out, ""
}

func (a *App) handleCreateCotinha(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title        string          `json:"title"`
		Description  string          `json:"description"`
		TotalCents   int64           `json:"total_cents"`
		SplitMode    string          `json:"split_mode"`
		Participants []participantIn `json:"participants"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 80 {
		writeError(w, 400, "Dê um nome de até 80 letras para a cotinha.")
		return
	}
	if utf8.RuneCountInString(in.Description) > 1000 {
		writeError(w, 400, "A descrição pode ter até 1000 letras.")
		return
	}
	if in.TotalCents <= 0 || in.TotalCents > 100_000_000 {
		writeError(w, 400, "Digite um valor válido.")
		return
	}
	if len(in.Participants) < 2 || len(in.Participants) > 50 {
		writeError(w, 400, "Escolha pelo menos um amigo para dividir.")
		return
	}
	seen := map[string]bool{}
	hasFriend := false
	for _, p := range in.Participants {
		if !uuidRe.MatchString(p.UserID) || seen[p.UserID] {
			writeError(w, 400, "Participantes inválidos.")
			return
		}
		seen[p.UserID] = true
		if p.UserID == me {
			continue
		}
		hasFriend = true
		ok, err := a.areFriends(ctx, me, p.UserID)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		if !ok {
			writeError(w, 403, "Você só pode criar cotinhas com seus amigos.")
			return
		}
	}
	if !hasFriend {
		writeError(w, 400, "Escolha pelo menos um amigo para dividir.")
		return
	}
	amounts, msg := splitShares(in.TotalCents, in.SplitMode, in.Participants)
	if msg != "" {
		writeError(w, 400, msg)
		return
	}

	tx, err := a.db.Begin(ctx)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO cotinhas (creditor_id, title, description, total_cents, split_mode)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`, me, in.Title, in.Description, in.TotalCents, in.SplitMode).Scan(&id)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	for i, p := range in.Participants {
		var weight *float64
		if in.SplitMode == "weight" {
			v := p.Value
			weight = &v
		}
		if _, err := tx.Exec(ctx, `INSERT INTO cotinha_shares (cotinha_id, user_id, amount_cents, weight) VALUES ($1, $2, $3, $4)`,
			id, p.UserID, amounts[i], weight); err != nil {
			a.serverError(w, r, err)
			return
		}
		if p.UserID != me && amounts[i] > 0 {
			if err := notify(ctx, tx, p.UserID, "cotinha_added", me, id, "", amounts[i]); err != nil {
				a.serverError(w, r, err)
				return
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

type CotinhaSummary struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	TotalCents  int64      `json:"total_cents"`
	CreatedAt   time.Time  `json:"created_at"`
	Creditor    PublicUser `json:"creditor"`
	Role        string     `json:"role"` // "creditor" or "debtor"
	MyShare     int64      `json:"my_share_cents"`
	Outstanding int64      `json:"outstanding_cents"` // creditor: still to receive; debtor: still to pay
	Settled     bool       `json:"settled"`
	Hidden      bool       `json:"hidden_from_stats"`
	People      int        `json:"people"`
	PhotoURL    string     `json:"photo_url"`
}

func (a *App) handleListCotinhas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me := currentUser(ctx)
	filter := r.URL.Query().Get("status")
	rows, err := a.db.Query(ctx, `
		SELECT c.id, c.title, c.total_cents, c.created_at, c.creditor_id,
			(SELECT count(*) FROM cotinha_shares s WHERE s.cotinha_id = c.id),
			coalesce((SELECT amount_cents FROM cotinha_shares s WHERE s.cotinha_id = c.id AND s.user_id = $1), 0),
			(SELECT object_key FROM cotinha_photos p WHERE p.cotinha_id = c.id ORDER BY created_at LIMIT 1),
			EXISTS (SELECT 1 FROM cotinha_stats_hidden h WHERE h.cotinha_id = c.id AND h.user_id = $1)
		FROM cotinhas c
		WHERE c.creditor_id = $1 OR EXISTS (SELECT 1 FROM cotinha_shares s WHERE s.cotinha_id = c.id AND s.user_id = $1)
		ORDER BY c.created_at DESC LIMIT 500`, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer rows.Close()
	type row struct {
		CotinhaSummary
		creditorID string
		photo      *string
	}
	var list []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ID, &x.Title, &x.TotalCents, &x.CreatedAt, &x.creditorID, &x.People, &x.MyShare, &x.photo, &x.Hidden); err != nil {
			a.serverError(w, r, err)
			return
		}
		list = append(list, x)
	}
	rows.Close()
	shares, err := sharesInvolving(ctx, a.db, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	out := []CotinhaSummary{}
	for _, x := range list {
		c := x.CotinhaSummary
		if x.creditorID == me {
			c.Role = "creditor"
		} else {
			c.Role = "debtor"
		}
		for _, s := range shares {
			if s.CotinhaID != c.ID {
				continue
			}
			if c.Role == "creditor" || s.DebtorID == me {
				c.Outstanding += s.Remaining()
			}
		}
		c.Settled = c.Outstanding == 0
		if filter == "open" && c.Settled || filter == "settled" && !c.Settled {
			continue
		}
		if c.Creditor, err = a.userByID(ctx, x.creditorID); err != nil {
			a.serverError(w, r, err)
			return
		}
		c.PhotoURL = a.store.URL(ctx, x.photo)
		out = append(out, c)
	}
	writeJSON(w, 200, out)
}

// canSeeCotinha reports whether the user is the creditor or a participant.
func (a *App) canSeeCotinha(ctx context.Context, id, user string) (bool, error) {
	var ok bool
	err := a.db.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM cotinhas c WHERE c.id = $1 AND (c.creditor_id = $2
			OR EXISTS (SELECT 1 FROM cotinha_shares s WHERE s.cotinha_id = c.id AND s.user_id = $2)))`, id, user).Scan(&ok)
	return ok, err
}

type ShareDetail struct {
	User       PublicUser `json:"user"`
	Amount     int64      `json:"amount_cents"`
	Confirmed  int64      `json:"confirmed_cents"`
	Pending    int64      `json:"pending_cents"`
	Remaining  int64      `json:"remaining_cents"`
	IsCreditor bool       `json:"is_creditor"`
}

type PhotoOut struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	UploadedBy string `json:"uploaded_by"`
}

func (a *App) handleGetCotinha(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	if ok, err := a.canSeeCotinha(ctx, id, me); err != nil {
		a.serverError(w, r, err)
		return
	} else if !ok {
		writeError(w, 404, "Cotinha não encontrada.")
		return
	}
	var c struct {
		ID          string    `json:"id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		TotalCents  int64     `json:"total_cents"`
		SplitMode   string    `json:"split_mode"`
		CreatedAt   time.Time `json:"created_at"`
	}
	var creditorID string
	err := a.db.QueryRow(ctx, `SELECT id, title, description, total_cents, split_mode, created_at, creditor_id FROM cotinhas WHERE id = $1`, id).
		Scan(&c.ID, &c.Title, &c.Description, &c.TotalCents, &c.SplitMode, &c.CreatedAt, &creditorID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	creditor, err := a.userByID(ctx, creditorID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	debts, err := sharesOfCotinha(ctx, a.db, id)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	rows, err := a.db.Query(ctx, `SELECT user_id, amount_cents FROM cotinha_shares WHERE cotinha_id = $1`, id)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	type rawShare struct {
		user   string
		amount int64
	}
	var raw []rawShare
	for rows.Next() {
		var s rawShare
		if err := rows.Scan(&s.user, &s.amount); err != nil {
			rows.Close()
			a.serverError(w, r, err)
			return
		}
		raw = append(raw, s)
	}
	rows.Close()
	shares := []ShareDetail{}
	var expected, received, pending int64
	for _, s := range raw {
		u, err := a.userByID(ctx, s.user)
		if err != nil {
			a.serverError(w, r, err)
			return
		}
		d := ShareDetail{User: u, Amount: s.amount, IsCreditor: s.user == creditorID}
		for _, debt := range debts {
			if debt.DebtorID == s.user {
				d.Confirmed, d.Pending, d.Remaining = debt.Confirmed, debt.Pending, debt.Remaining()
				expected += debt.Amount
				received += debt.Confirmed
				pending += debt.Pending
			}
		}
		shares = append(shares, d)
	}
	sort.SliceStable(shares, func(i, j int) bool {
		if shares[i].IsCreditor != shares[j].IsCreditor {
			return shares[i].IsCreditor
		}
		return shares[i].User.Name < shares[j].User.Name
	})

	prow, err := a.db.Query(ctx, `SELECT id, object_key, uploaded_by FROM cotinha_photos WHERE cotinha_id = $1 ORDER BY created_at`, id)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	photos := []PhotoOut{}
	for prow.Next() {
		var p PhotoOut
		var key string
		if err := prow.Scan(&p.ID, &key, &p.UploadedBy); err != nil {
			prow.Close()
			a.serverError(w, r, err)
			return
		}
		p.URL = a.store.URL(ctx, &key)
		photos = append(photos, p)
	}
	prow.Close()

	payments, err := a.loadPayments(ctx, `WHERE p.id IN (SELECT payment_id FROM payment_allocations WHERE cotinha_id = $1)
		AND (p.payer_id = $2 OR p.payee_id = $2 OR $2 = $3) ORDER BY p.created_at DESC`, id, me, creditorID)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	// Only show each payment's part that belongs to this cotinha.
	history := []map[string]any{}
	for _, p := range payments {
		var part int64
		for _, al := range p.Cotinhas {
			if al.CotinhaID == id {
				part = al.Amount
			}
		}
		history = append(history, map[string]any{
			"payment_id": p.ID, "payer": p.Payer, "amount_cents": part, "method": p.Method,
			"status": p.Status, "created_at": p.CreatedAt, "decided_at": p.DecidedAt,
		})
	}

	var hidden bool
	if err := a.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cotinha_stats_hidden WHERE user_id = $1 AND cotinha_id = $2)`, me, id).Scan(&hidden); err != nil {
		a.serverError(w, r, err)
		return
	}
	hasPayments, err := cotinhaHasPayments(ctx, a.db, id)
	if err != nil {
		a.serverError(w, r, err)
		return
	}

	writeJSON(w, 200, map[string]any{
		"hidden_from_stats": hidden,
		"can_delete":        creditorID == me && !hasPayments,
		"has_payments":      hasPayments,
		"cotinha":           c,
		"creditor":          creditor,
		"shares":            shares,
		"expected_cents":    expected,
		"received_cents":    received,
		"pending_cents":     pending,
		"settled":           received == expected,
		"photos":            photos,
		"history":           history,
	})
}

func (a *App) handleUploadPhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	if ok, err := a.canSeeCotinha(ctx, id, me); err != nil {
		a.serverError(w, r, err)
		return
	} else if !ok {
		writeError(w, 404, "Cotinha não encontrada.")
		return
	}
	// Only whoever created the cotinha (the creditor) adds pictures.
	var isCreditor bool
	if err := a.db.QueryRow(ctx, `SELECT creditor_id = $2 FROM cotinhas WHERE id = $1`, id, me).Scan(&isCreditor); err != nil {
		a.serverError(w, r, err)
		return
	}
	if !isCreditor {
		writeError(w, 403, "Só quem criou a cotinha pode adicionar fotos.")
		return
	}
	var count int
	if err := a.db.QueryRow(ctx, `SELECT count(*) FROM cotinha_photos WHERE cotinha_id = $1`, id).Scan(&count); err != nil {
		a.serverError(w, r, err)
		return
	}
	if count >= maxPhotos {
		writeError(w, 409, "Cada cotinha pode ter até 6 fotos.")
		return
	}
	data, ctype, ext, ok := readImage(w, r, photoSide)
	if !ok {
		return
	}
	key, err := a.store.Put(ctx, "cotinhas/"+id, ext, ctype, data)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	var p PhotoOut
	err = a.db.QueryRow(ctx, `INSERT INTO cotinha_photos (cotinha_id, object_key, uploaded_by) VALUES ($1, $2, $3) RETURNING id`, id, key, me).Scan(&p.ID)
	if err != nil {
		a.store.Delete(ctx, key)
		a.serverError(w, r, err)
		return
	}
	p.URL, p.UploadedBy = a.store.URL(ctx, &key), me
	writeJSON(w, 201, p)
}

func (a *App) handleDeletePhoto(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	photoID, ok := pathID(w, r, "photoId")
	if !ok {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	var key string
	err := a.db.QueryRow(ctx, `DELETE FROM cotinha_photos p USING cotinhas c
		WHERE p.id = $1 AND p.cotinha_id = $2 AND c.id = p.cotinha_id AND c.creditor_id = $3
		RETURNING p.object_key`, photoID, id, me).Scan(&key)
	if isNoRows(err) {
		writeError(w, 404, "Foto não encontrada.")
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	a.store.Delete(ctx, key)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// cotinhaHasPayments reports whether any payment (pending, confirmed or a
// compensation) already counts towards the cotinha.
func cotinhaHasPayments(ctx context.Context, q querier, id string) (bool, error) {
	var has bool
	err := q.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM payment_allocations pa JOIN payments p ON p.id = pa.payment_id
		WHERE pa.cotinha_id = $1 AND p.status IN ('pending', 'confirmed'))`, id).Scan(&has)
	return has, err
}

// handleDeleteCotinha lets the creator delete a cotinha nobody has paid yet.
// Participants are told; the photos are removed from storage.
func (a *App) handleDeleteCotinha(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
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
	var creditor, title string
	err = tx.QueryRow(ctx, `SELECT creditor_id, title FROM cotinhas WHERE id = $1 FOR UPDATE`, id).Scan(&creditor, &title)
	if isNoRows(err) {
		writeError(w, 404, "Cotinha não encontrada.")
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	if creditor != me {
		writeError(w, 403, "Só quem criou a cotinha pode apagá-la.")
		return
	}
	if has, err := cotinhaHasPayments(ctx, tx, id); err != nil {
		a.serverError(w, r, err)
		return
	} else if has {
		writeError(w, 409, "Essa cotinha já tem pagamentos registrados e não pode ser apagada.")
		return
	}
	rows, err := tx.Query(ctx, `SELECT user_id, amount_cents FROM cotinha_shares WHERE cotinha_id = $1 AND user_id <> $2`, id, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	type part struct {
		user   string
		amount int64
	}
	var parts []part
	for rows.Next() {
		var p part
		if err := rows.Scan(&p.user, &p.amount); err != nil {
			rows.Close()
			a.serverError(w, r, err)
			return
		}
		parts = append(parts, p)
	}
	rows.Close()
	var keys []string
	krows, err := tx.Query(ctx, `SELECT object_key FROM cotinha_photos WHERE cotinha_id = $1`, id)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	for krows.Next() {
		var k string
		if err := krows.Scan(&k); err != nil {
			krows.Close()
			a.serverError(w, r, err)
			return
		}
		keys = append(keys, k)
	}
	krows.Close()
	if _, err := tx.Exec(ctx, `DELETE FROM cotinhas WHERE id = $1`, id); err != nil {
		a.serverError(w, r, err)
		return
	}
	for _, p := range parts {
		if err := notifyTitled(ctx, tx, p.user, "cotinha_deleted", me, "", "", p.amount, title); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	if err := tx.Commit(ctx); err != nil {
		a.serverError(w, r, err)
		return
	}
	for _, k := range keys {
		a.store.Delete(ctx, k)
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// handleStatsHidden sets whether a cotinha counts in the current user's statistics.
func (a *App) handleStatsHidden(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Hidden bool `json:"hidden"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	if ok, err := a.canSeeCotinha(ctx, id, me); err != nil {
		a.serverError(w, r, err)
		return
	} else if !ok {
		writeError(w, 404, "Cotinha não encontrada.")
		return
	}
	var err error
	if in.Hidden {
		_, err = a.db.Exec(ctx, `INSERT INTO cotinha_stats_hidden (user_id, cotinha_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, me, id)
	} else {
		_, err = a.db.Exec(ctx, `DELETE FROM cotinha_stats_hidden WHERE user_id = $1 AND cotinha_id = $2`, me, id)
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"hidden": in.Hidden})
}
