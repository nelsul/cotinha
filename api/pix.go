package main

import (
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	digitsRe = regexp.MustCompile(`\D`)
	evpRe    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// normalizePixKey validates a Pix key for its type and returns the canonical form.
func normalizePixKey(kind, key string) (string, bool) {
	key = strings.TrimSpace(key)
	switch kind {
	case "cpf":
		d := digitsRe.ReplaceAllString(key, "")
		return d, validCPF(d)
	case "cnpj":
		d := digitsRe.ReplaceAllString(key, "")
		return d, validCNPJ(d)
	case "email":
		k := strings.ToLower(key)
		addr, err := mail.ParseAddress(k)
		return k, err == nil && addr.Address == k && len(k) <= 77
	case "phone":
		d := digitsRe.ReplaceAllString(key, "")
		if len(d) == 10 || len(d) == 11 {
			d = "55" + d
		}
		return "+" + d, strings.HasPrefix(d, "55") && (len(d) == 12 || len(d) == 13)
	case "random":
		k := strings.ToLower(key)
		return k, evpRe.MatchString(k)
	}
	return "", false
}

func validCPF(d string) bool {
	if len(d) != 11 || strings.Count(d, string(d[0])) == 11 {
		return false
	}
	check := func(n int) int {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(d[i]-'0') * (n + 1 - i)
		}
		r := sum * 10 % 11
		if r == 10 {
			r = 0
		}
		return r
	}
	return check(9) == int(d[9]-'0') && check(10) == int(d[10]-'0')
}

func validCNPJ(d string) bool {
	if len(d) != 14 || strings.Count(d, string(d[0])) == 14 {
		return false
	}
	check := func(n int) int {
		weights := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}[13-n:]
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(d[i]-'0') * weights[i]
		}
		r := sum % 11
		if r < 2 {
			return 0
		}
		return 11 - r
	}
	return check(12) == int(d[12]-'0') && check(13) == int(d[13]-'0')
}

// emv formats one EMV field: id + 2-digit length + value.
func emv(id, value string) string { return fmt.Sprintf("%s%02d%s", id, len(value), value) }

// pixASCII strips accents and keeps the characters banks accept in name/city fields.
func pixASCII(s string, max int) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ') {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	out := strings.TrimSpace(b.String())
	if len(out) > max {
		out = out[:max]
	}
	if out == "" {
		out = "COTINHA"
	}
	return out
}

func crc16(s string) string {
	crc := uint16(0xFFFF)
	for i := 0; i < len(s); i++ {
		crc ^= uint16(s[i]) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return fmt.Sprintf("%04X", crc)
}

// pixPayload builds a static Pix "copia e cola" (BR Code) with a fixed amount.
func pixPayload(key, name, txid string, cents int64) string {
	account := emv("00", "br.gov.bcb.pix") + emv("01", key)
	p := emv("00", "01") +
		emv("26", account) +
		emv("52", "0000") +
		emv("53", "986") +
		emv("54", fmt.Sprintf("%d.%02d", cents/100, cents%100)) +
		emv("58", "BR") +
		emv("59", pixASCII(name, 25)) +
		emv("60", "BRASIL") +
		emv("62", emv("05", txid)) +
		"6304"
	return p + crc16(p)
}

// handlePixCode returns the Pix code to pay a friend a given amount.
func (a *App) handlePixCode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	me := currentUser(ctx)
	cents, err := strconv.ParseInt(r.URL.Query().Get("amount_cents"), 10, 64)
	if err != nil || cents <= 0 || cents > 100_000_000 {
		writeError(w, 400, "Valor inválido.")
		return
	}
	if friends, err := a.areFriends(ctx, me, id); err != nil {
		a.serverError(w, r, err)
		return
	} else if !friends {
		writeError(w, 403, "Vocês não são amigos no Cotinha.")
		return
	}
	var name string
	var key, kind *string
	if err := a.db.QueryRow(ctx, `SELECT name, pix_key, pix_key_type FROM users WHERE id = $1`, id).Scan(&name, &key, &kind); err != nil {
		a.serverError(w, r, err)
		return
	}
	if key == nil || *key == "" {
		writeError(w, 409, "Essa pessoa ainda não cadastrou uma chave Pix.")
		return
	}
	writeJSON(w, 200, map[string]string{
		"payload":      pixPayload(*key, name, "COTINHA", cents),
		"pix_key":      maskPixKey(*kind, *key),
		"pix_key_type": *kind,
		"name":         name,
	})
}

// maskPixKey hides part of a key so friends can recognise it without copying it whole.
func maskPixKey(kind, key string) string {
	switch kind {
	case "cpf":
		return "***." + key[3:6] + "." + key[6:9] + "-**"
	case "cnpj":
		return key[:2] + ".***.***/" + key[8:12] + "-**"
	case "email":
		at := strings.Index(key, "@")
		if at <= 2 {
			return key
		}
		return key[:2] + strings.Repeat("•", 4) + key[at:]
	case "phone":
		if len(key) > 4 {
			return "(**) *****-" + key[len(key)-4:]
		}
	case "random":
		return key[:8] + "-••••"
	}
	return key
}
