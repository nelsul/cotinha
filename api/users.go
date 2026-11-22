package main

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type PublicUser struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Handle    string `json:"handle"`
	AvatarURL string `json:"avatar_url"`
}

type Me struct {
	PublicUser
	Email      string `json:"email"`
	PixKeyType string `json:"pix_key_type"`
	PixKey     string `json:"pix_key"`
	Onboarded  bool   `json:"onboarded"`
}

func (a *App) loadMe(ctx context.Context, id string) (Me, error) {
	var m Me
	var handle, avatar, pixType, pix *string
	err := a.db.QueryRow(ctx, `SELECT id, email, name, handle, avatar_key, pix_key_type, pix_key FROM users WHERE id = $1`, id).
		Scan(&m.ID, &m.Email, &m.Name, &handle, &avatar, &pixType, &pix)
	if err != nil {
		return m, err
	}
	m.Handle = deref(handle)
	m.AvatarURL = a.store.URL(ctx, avatar)
	m.PixKeyType = deref(pixType)
	m.PixKey = deref(pix)
	m.Onboarded = m.Name != "" && m.Handle != "" && m.PixKey != ""
	return m, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// publicUser builds the JSON for another user from scanned columns.
func (a *App) publicUser(ctx context.Context, id, name string, handle, avatar *string) PublicUser {
	return PublicUser{ID: id, Name: name, Handle: deref(handle), AvatarURL: a.store.URL(ctx, avatar)}
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	me, err := a.loadMe(r.Context(), currentUser(r.Context()))
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 200, me)
}

var handleRe = regexp.MustCompile(`^[a-z0-9._]{3,20}$`)

func (a *App) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name       *string `json:"name"`
		Handle     *string `json:"handle"`
		PixKeyType *string `json:"pix_key_type"`
		PixKey     *string `json:"pix_key"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	uid := currentUser(ctx)
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || utf8.RuneCountInString(name) > 40 {
			writeError(w, 400, "Digite um nome de até 40 letras.")
			return
		}
		if _, err := a.db.Exec(ctx, `UPDATE users SET name = $1 WHERE id = $2`, name, uid); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	if in.Handle != nil {
		h := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(*in.Handle), "@"))
		if !handleRe.MatchString(h) {
			writeError(w, 400, "O @ precisa ter de 3 a 20 caracteres: letras minúsculas, números, ponto ou _.")
			return
		}
		var taken bool
		if err := a.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE handle = $1 AND id <> $2)`, h, uid).Scan(&taken); err != nil {
			a.serverError(w, r, err)
			return
		}
		if taken {
			writeError(w, 409, "Esse @ já está em uso.")
			return
		}
		if _, err := a.db.Exec(ctx, `UPDATE users SET handle = $1 WHERE id = $2`, h, uid); err != nil {
			// Two people can pick the same free @ at the same moment; the unique index decides.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeError(w, 409, "Esse @ já está em uso.")
				return
			}
			a.serverError(w, r, err)
			return
		}
	}
	if in.PixKeyType != nil || in.PixKey != nil {
		if in.PixKeyType == nil || in.PixKey == nil {
			writeError(w, 400, "Informe o tipo e a chave Pix.")
			return
		}
		key, ok := normalizePixKey(*in.PixKeyType, *in.PixKey)
		if !ok {
			writeError(w, 400, "Essa chave Pix não parece válida para o tipo escolhido.")
			return
		}
		if _, err := a.db.Exec(ctx, `UPDATE users SET pix_key_type = $1, pix_key = $2 WHERE id = $3`, *in.PixKeyType, key, uid); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	a.handleMe(w, r)
}

// handleHandleAvailable lets the onboarding and profile forms check a @ while it is typed.
func (a *App) handleHandleAvailable(w http.ResponseWriter, r *http.Request) {
	h := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(r.URL.Query().Get("handle")), "@"))
	if !handleRe.MatchString(h) {
		writeJSON(w, 200, map[string]any{"handle": h, "valid": false, "available": false})
		return
	}
	var takenBy *string
	err := a.db.QueryRow(r.Context(), `SELECT id FROM users WHERE handle = $1`, h).Scan(&takenBy)
	if err != nil && !isNoRows(err) {
		a.serverError(w, r, err)
		return
	}
	available := takenBy == nil || *takenBy == currentUser(r.Context())
	writeJSON(w, 200, map[string]any{"handle": h, "valid": true, "available": available})
}

func (a *App) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	data, ctype, ext, ok := readImage(w, r, avatarSide)
	if !ok {
		return
	}
	ctx := r.Context()
	uid := currentUser(ctx)
	key, err := a.store.Put(ctx, "avatars/"+uid, ext, ctype, data)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	var old *string
	if err := a.db.QueryRow(ctx, `SELECT avatar_key FROM users WHERE id = $1`, uid).Scan(&old); err != nil {
		a.serverError(w, r, err)
		return
	}
	if _, err := a.db.Exec(ctx, `UPDATE users SET avatar_key = $1 WHERE id = $2`, key, uid); err != nil {
		a.serverError(w, r, err)
		return
	}
	if old != nil {
		a.store.Delete(ctx, *old)
	}
	a.handleMe(w, r)
}

// relation describes how the current user relates to another one.
func (a *App) relation(ctx context.Context, me, other string) (string, error) {
	if me == other {
		return "self", nil
	}
	friends, err := a.areFriends(ctx, me, other)
	if err != nil || friends {
		return "friend", err
	}
	var rel string
	err = a.db.QueryRow(ctx, `
		SELECT CASE WHEN from_id = $1 THEN 'request_sent' ELSE 'request_received' END
		FROM friend_requests WHERE status = 'pending'
		  AND ((from_id = $1 AND to_id = $2) OR (from_id = $2 AND to_id = $1))
		LIMIT 1`, me, other).Scan(&rel)
	if isNoRows(err) {
		return "none", nil
	}
	return rel, err
}

type UserWithRelation struct {
	PublicUser
	Relation string `json:"relation"`
}

func (a *App) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(r.URL.Query().Get("q")), "@"))
	if len(q) < 2 {
		writeJSON(w, 200, []UserWithRelation{})
		return
	}
	q = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
	ctx := r.Context()
	me := currentUser(ctx)
	rows, err := a.db.Query(ctx, `
		SELECT id, name, handle, avatar_key FROM users
		WHERE handle LIKE $1 || '%' AND id <> $2 AND name <> ''
		ORDER BY length(handle), handle LIMIT 10`, q, me)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	defer rows.Close()
	out := []UserWithRelation{}
	for rows.Next() {
		var id, name string
		var handle, avatar *string
		if err := rows.Scan(&id, &name, &handle, &avatar); err != nil {
			a.serverError(w, r, err)
			return
		}
		out = append(out, UserWithRelation{PublicUser: a.publicUser(ctx, id, name, handle, avatar)})
	}
	rows.Close()
	for i := range out {
		if out[i].Relation, err = a.relation(ctx, me, out[i].ID); err != nil {
			a.serverError(w, r, err)
			return
		}
	}
	writeJSON(w, 200, out)
}

func (a *App) handleUserByHandle(w http.ResponseWriter, r *http.Request) {
	h := strings.ToLower(strings.TrimPrefix(chi.URLParam(r, "handle"), "@"))
	ctx := r.Context()
	var id, name string
	var handle, avatar *string
	err := a.db.QueryRow(ctx, `SELECT id, name, handle, avatar_key FROM users WHERE handle = $1`, h).Scan(&id, &name, &handle, &avatar)
	if isNoRows(err) {
		writeError(w, 404, "Ninguém usa esse @.")
		return
	} else if err != nil {
		a.serverError(w, r, err)
		return
	}
	rel, err := a.relation(ctx, currentUser(ctx), id)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	writeJSON(w, 200, UserWithRelation{PublicUser: a.publicUser(ctx, id, name, handle, avatar), Relation: rel})
}
