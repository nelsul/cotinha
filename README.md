# Cotinha

Split bills with friends: shared expenses ("cotinhas") split equally, by amount or by weight, partial payments, cancelling out mutual debts, and payments that only close when the person who is owed confirms them.

Live at **https://cotinha.nelsul.com**. Dark only, mobile first. The app's interface is in Brazilian Portuguese; code and docs are in English.

## How it works

- **Login** with a 6-digit code sent by email (no passwords).
- Everyone has a unique **@handle** (checked live while it is typed) and a **Pix key**. Friends find each other by @ or by a QR code (`/add/<handle>`). Requests can be accepted, declined or blocked for 24 hours.
- A **cotinha** is created by whoever paid, only with friends, split `equal`, by `amount` or by `weight`. It can have a description and up to 6 photos.
- **Paying (MVP):** the debtor pays by Pix straight to the creditor's key (the app shows a QR and a copy-paste code with the amount), attaches a receipt and marks it as paid. The payment stays `pending` until the **creditor confirms** it. Partial payments are allowed.
- **Compensation:** when two people owe each other, one tap cancels the smaller amount on both sides.
- **History and statistics:** `/cotinhas` lists every cotinha (open, settled, search). Each person can leave a cotinha out of their own statistics. The creator can delete a cotinha while nobody has paid towards it; participants are notified.
- **Notifications** live in the app (`/notificacoes`, the bell on the home screen). The only emails Cotinha sends are login codes.
- Ready for later: `payments.method` already accepts `pix_auto`, `debit`, `credit` and `apple_pay`, with `fee_cents`, `provider`, `provider_ref` and `confirmed_by = webhook`. The receiver always gets the exact amount; any fee is added on top for the payer.

## Stack

| Part | What |
|---|---|
| `api/` | Go (chi, pgx). Migrations in `api/migrations` are embedded and applied when the API starts. It also serves the built web app. |
| `web/` | SvelteKit 2 / Svelte 5 as a static single-page app. |
| Database | Postgres 17 |
| Pictures | S3-compatible bucket (Cloudflare R2 in production), private, served through short-lived presigned URLs |

**Pictures are kept small.** The browser resizes before uploading (longest side 1600 px, WebP or JPEG at ~80% quality); profile photos are cropped in a circle editor and saved as 512×512. The API checks again and re-encodes anything larger as JPEG (`api/image.go`). Re-encoded pictures carry no EXIF metadata, such as GPS location. HEIC files that a browser cannot decode are stored as uploaded.

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
