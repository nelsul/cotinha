# AGENTS.md

Guide for AI coding agents working in this repo. Humans: start with [README.md](README.md) and [CONTRIBUTING.md](CONTRIBUTING.md); this file sums up what an agent needs.

## The project

Cotinha splits bills between friends: shared expenses ("cotinhas") split equally, by amount or by weight, paid by Pix straight to the creditor's key, with receipts and the creditor's confirmation. Live at https://cotinha.nelsul.com. The owner is Nelson.

| Folder | What | Stack |
|---|---|---|
| `api/` | JSON API under `/api`, also serves the built web app | Go, chi, pgx, PostgreSQL 17, S3-compatible storage (Cloudflare R2 in production) |
| `web/` | The app people use | SvelteKit 2 (static SPA), Svelte 5 runes, TypeScript |
| `.github/workflows/` | `ci.yml` (`check`) and `deploy.yml` | GitHub Actions, deploy over SSH to Docker on a VPS |

Key ideas: money is integer cents; a payment from A to B is spread over A's open shares in B's cotinhas (`payment_allocations`, oldest first); a payment is `pending` until the payee confirms it; `payments.method` is ready for `pix_auto`, `debit`, `credit` and `apple_pay`, but the app only creates `pix_manual` and `compensation` today. Notifications live in the app only; the only emails sent are login codes.

## Rules

### Secrets and privacy (most important)

- **Never commit or print secrets**: `.env`, R2 keys, SMTP credentials, the database password, server addresses, SSH keys. The repo is **public**.
- Never put real people's email addresses or Pix keys in code, tests, screenshots, commits, issues or PRs. Test people use `@example.test`.
- Before every push, check the diff and new files for anything secret-looking.

### Workflow

- `main` is protected: work on a branch and open a PR. Never push to `main`.
- **Merging a PR that touches `api/`, `web/`, `Dockerfile`, `docker-compose.yml` or the workflows deploys to production.** Treat a merge as a deploy.
- CI's `check` job must pass: `gofmt`, `go vet`, `go test`, `npm run check`, `npm run build`.
- Merge with squash. Commit messages and PR descriptions are in English.

### Test locally first

- Run changes against the local Docker Postgres and local S3 (`docker-compose.dev.yml`, `./dev.sh`, `npm run dev` in `web/`).
- **Never send real emails while testing.** Without `SMTP_HOST` the login code is printed in the API's output.
- **Never run migrations or test writes against production.** Production applies `api/migrations/*.sql` by itself when the API starts. Add new numbered files; never edit one already on `main`.

### Language and look

- Everything people see is in **Portuguese (pt-BR)**, API error messages included. Code, comments, commits and docs are in English.
- **Dark only**, the "Roda" look: tokens in `web/src/app.css`, icons in `web/src/lib/components/Icon.svelte`. No new colors or emoji in the UI.
- Mobile first: check at ~390 px; nothing should scroll sideways. Buttons at least 44 px.
