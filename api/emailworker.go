package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// A free Gmail account can send about 500 emails per rolling 24 hours, and
// Google blocks sending for a while when that is exceeded. Notifications stay
// under mailDailyLimit and leave loginReserve of it for login codes, so people
// can always sign in. Anything over the limit waits for the next window.
const (
	mailDailyLimit  = 450
	loginReserve    = 50
	mailBatch       = 20
	mailPoll        = 20 * time.Second
	notificationTTL = 48 * time.Hour // older unsent notifications are not emailed
)

func (a *App) emailsSentToday(ctx context.Context) (int, error) {
	var n int
	err := a.db.QueryRow(ctx, `SELECT count(*) FROM email_log WHERE sent_at > now() - interval '24 hours'`).Scan(&n)
	return n, err
}

func (a *App) logEmail(ctx context.Context, kind string) {
	if _, err := a.db.Exec(ctx, `INSERT INTO email_log (kind) VALUES ($1)`, kind); err != nil {
		a.log.Error("email log", "err", err)
	}
}

// emailWorker sends one email per notification, in the background, until ctx ends.
func (a *App) emailWorker(ctx context.Context) {
	t := time.NewTicker(mailPoll)
	defer t.Stop()
	for {
		if err := a.sendPendingEmails(ctx); err != nil && ctx.Err() == nil {
			a.log.Error("email worker", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

type pendingEmail struct {
	id, kind, to, actor, title, paymentID, cotinhaID, actorID string
	amount                                                    int64
	optedOut, read, stale                                     bool
}

func (a *App) sendPendingEmails(ctx context.Context) error {
	sent, err := a.emailsSentToday(ctx)
	if err != nil {
		return err
	}
	room := mailDailyLimit - loginReserve - sent
	if room <= 0 {
		return nil
	}
	rows, err := a.db.Query(ctx, `
		SELECT n.id, n.kind, u.email, coalesce(act.name, ''), coalesce(c.title, n.title, ''),
			coalesce(n.payment_id::text, ''), coalesce(n.cotinha_id::text, ''), coalesce(n.actor_id::text, ''),
			coalesce(n.amount_cents, 0), NOT u.email_notifications, n.read_at IS NOT NULL,
			n.created_at < $2
		FROM notifications n
		JOIN users u ON u.id = n.user_id
		LEFT JOIN users act ON act.id = n.actor_id
		LEFT JOIN cotinhas c ON c.id = n.cotinha_id
		WHERE n.email_state IS NULL
		ORDER BY n.created_at LIMIT $1`, min(room, mailBatch), time.Now().Add(-notificationTTL))
	if err != nil {
		return err
	}
	var list []pendingEmail
	for rows.Next() {
		var p pendingEmail
		if err := rows.Scan(&p.id, &p.kind, &p.to, &p.actor, &p.title, &p.paymentID, &p.cotinhaID, &p.actorID,
			&p.amount, &p.optedOut, &p.read, &p.stale); err != nil {
			rows.Close()
			return err
		}
		list = append(list, p)
	}
	rows.Close()

	for _, p := range list {
		state := "skipped"
		// Already seen in the app, turned off by the user, or too old to matter.
		if !p.optedOut && !p.read && !p.stale {
			subject, body := a.notificationEmail(p)
			if err := a.mail.Send(p.to, subject, body); err != nil {
				a.log.Error("notification email", "id", p.id, "err", err)
				state = "failed"
			} else {
				state = "sent"
				a.logEmail(ctx, "notification")
			}
		}
		if _, err := a.db.Exec(ctx, `UPDATE notifications SET email_state = $1, emailed_at = CASE WHEN $1 = 'sent' THEN now() END WHERE id = $2`, state, p.id); err != nil {
			return err
		}
	}
	return nil
}

func firstWord(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return "Alguém"
}

func brl(cents int64) string {
	reais := fmt.Sprintf("%d", cents/100)
	var b strings.Builder
	for i, r := range reais {
		if i > 0 && (len(reais)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("R$ %s,%02d", b.String(), cents%100)
}

// notificationEmail writes the subject and body for one notification, in pt-BR.
func (a *App) notificationEmail(p pendingEmail) (string, string) {
	who := firstWord(p.actor)
	amount := brl(p.amount)
	link := a.cfg.PublicURL + "/notificacoes"
	var subject, text string
	switch p.kind {
	case "friend_request":
		subject = who + " quer ser seu amigo no Cotinha"
		text = who + " te mandou um pedido de amizade no Cotinha."
		link = a.cfg.PublicURL + "/amigos"
	case "friend_accepted":
		subject = who + " aceitou seu pedido de amizade"
		text = who + " aceitou seu pedido. Agora vocês podem dividir cotinhas."
		link = a.cfg.PublicURL + "/amigos"
	case "cotinha_added":
		subject = fmt.Sprintf("%s criou “%s”: sua parte é %s", who, p.title, amount)
		text = fmt.Sprintf("%s criou a cotinha “%s” com você. Sua parte é %s.", who, p.title, amount)
		link = a.cfg.PublicURL + "/cotinha/" + p.cotinhaID
	case "cotinha_deleted":
		subject = fmt.Sprintf("%s apagou a cotinha “%s”", who, p.title)
		text = fmt.Sprintf("%s apagou a cotinha “%s”. Você não deve mais os %s dela.", who, p.title, amount)
	case "payment_sent":
		subject = fmt.Sprintf("%s diz que te pagou %s", who, amount)
		text = fmt.Sprintf("%s marcou %s como pago para você. Confira no seu banco e confirme no Cotinha.", who, amount)
		link = a.cfg.PublicURL + "/pagamento/" + p.paymentID
	case "payment_confirmed":
		subject = fmt.Sprintf("%s confirmou seu pagamento de %s", who, amount)
		text = fmt.Sprintf("%s confirmou que recebeu os %s que você pagou.", who, amount)
		link = a.cfg.PublicURL + "/pagamento/" + p.paymentID
	case "payment_rejected":
		subject = fmt.Sprintf("%s não recebeu seu Pix de %s", who, amount)
		text = fmt.Sprintf("%s disse que o Pix de %s não caiu na conta. Confira e tente de novo.", who, amount)
		link = a.cfg.PublicURL + "/pagamento/" + p.paymentID
	case "compensated":
		subject = fmt.Sprintf("%s compensou as dívidas entre vocês", who)
		text = fmt.Sprintf("%s compensou %s de cada lado entre vocês no Cotinha.", who, amount)
		link = a.cfg.PublicURL + "/pagar/" + p.actorID
	default:
		subject, text = "Novidade no Cotinha", "Você tem uma novidade no Cotinha."
	}
	body := text + "\r\n\r\nAbrir no Cotinha: " + link +
		"\r\n\r\n—\r\nPara não receber estes e-mails, desligue \"Avisos por e-mail\" no seu perfil do Cotinha.\r\n"
	return subject, body
}
