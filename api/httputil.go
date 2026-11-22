package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type ctxKey int

const userKey ctxKey = 1

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError sends {"error": msg}. Messages are shown to users, so they are in Portuguese.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (a *App) serverError(w http.ResponseWriter, r *http.Request, err error) {
	a.log.Error("request failed", "path", r.URL.Path, "err", err)
	writeError(w, 500, "Algo deu errado. Tente de novo.")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, 400, "Requisição inválida.")
		return false
	}
	return true
}

func currentUser(ctx context.Context) string {
	id, _ := ctx.Value(userKey).(string)
	return id
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// pathID returns a validated UUID URL parameter, or writes a 404.
func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := chi.URLParam(r, name)
	if !uuidRe.MatchString(id) {
		writeError(w, 404, "Não encontrado.")
		return "", false
	}
	return id, true
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// orderedPair returns the two ids sorted, as stored in friendships.
func orderedPair(a, b string) (string, string) {
	if a < b {
		return a, b
	}
	return b, a
}

func (a *App) areFriends(ctx context.Context, x, y string) (bool, error) {
	u1, u2 := orderedPair(x, y)
	var ok bool
	err := a.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM friendships WHERE user_a = $1 AND user_b = $2)`, u1, u2).Scan(&ok)
	return ok, err
}
