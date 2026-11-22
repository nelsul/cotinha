# Cotinha

Divida contas com os amigos: cotinhas iguais, por valor ou por peso, pagamentos parciais, compensação de dívidas entre duas pessoas e confirmação de quem recebeu.

Live at **https://cotinha.nelsul.com**. Dark only, mobile first.

## How it works

- **Login** with a 6-digit code sent by email (no passwords).
- Everyone has a **@handle** and a **Pix key**. Friends find each other by @ or by a QR code (`/add/<handle>`). Requests can be accepted, declined or blocked for 24 hours.
- A **cotinha** is created by whoever paid, only with friends, split `equal`, by `amount` or by `weight`. It can have a description and up to 6 photos.
- **Paying (MVP):** the debtor pays by Pix straight to the creditor's key (the app shows a QR and a copy-paste code with the amount), attaches a receipt and marks it as paid. The payment stays `pending` until the **creditor confirms** it. Partial payments are allowed.
- **Compensation:** when two people owe each other, one tap cancels the smaller amount on both sides.
- Ready for later: `payments.method` already accepts `pix_auto`, `debit`, `credit` and `apple_pay`, with `fee_cents`, `provider`, `provider_ref` and `confirmed_by = webhook`. The receiver always gets the exact amount; any fee is added on top for the payer.

## Stack

| Part | What |
|---|---|
| `api/` | Go (chi, pgx). Migrations in `api/migrations` are embedded and applied when the API starts. It also serves the built web app. |
| `web/` | SvelteKit 2 / Svelte 5 as a static single-page app. |
| Database | Postgres 17 |
| Pictures | S3-compatible bucket (Cloudflare R2 in production), private, served through short-lived presigned URLs |

Money is always integer cents. A payment from A to B is spread over A's open shares in B's cotinhas (`payment_allocations`), oldest first.

## Develop locally

Requires Docker, Go and Node 22+.

```bash
docker compose -f docker-compose.dev.yml up -d   # Postgres on :5434, local S3 (SeaweedFS) on :8333
./dev.sh                                          # API on :8080; login codes are printed here
cd web && npm install && npm run dev              # app on http://localhost:5173
```

Use test emails at `@example.test`. Checks before a pull request:

```bash
cd api && gofmt -l . && go vet ./... && go test ./...
cd web && npm run check && npm run build
```

## Deploy

`main` is protected: every change goes through a pull request with the `check` CI job. Merging into `main` runs **Deploy**, which writes `.env` from the `production` environment secrets, ships the code to the VPS over SSH and rebuilds the containers, then runs a smoke test.

Production secrets (GitHub → Settings → Environments → `production`): `POSTGRES_PASSWORD`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `MAIL_FROM`, `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, plus `DEPLOY_SSH_KEY`, `DEPLOY_KNOWN_HOSTS`, `DEPLOY_HOST`.
