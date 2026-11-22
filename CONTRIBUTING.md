# Contributing

Thanks for helping build Cotinha! 💸

## Workflow

`main` is protected: nobody pushes to it directly, Nelson included. Every change goes through a pull request, and **merging into `main` deploys to https://cotinha.nelsul.com automatically**.

1. Create a branch from `main`: `git switch -c feature/short-name`. Fork the repo first if you don't have write access.
2. Make your change and run it locally (see [Run it locally](#run-it-locally)).
3. Run the checks for what you touched:
   - API: `gofmt -l .` (must print nothing), `go vet ./...` and `go test ./...` in `api/`
   - website: `npm run check` and `npm run build` in `web/`

   CI runs all of them on your PR as the `check` job, and it must pass before the PR can be merged.
4. Open a pull request with a short description in English. Add screenshots for anything visual, at phone size (about 390 px wide).
5. Once it's merged, the Deploy workflow builds the image, ships it to the server and runs a smoke test. Watch it in the Actions tab.

## Run it locally

You need Docker, Go (the version in `api/go.mod`) and Node 22+.

```bash
docker compose -f docker-compose.dev.yml up -d   # Postgres on :5434, a local S3 (SeaweedFS) on :8333
./dev.sh                                          # API on :8080; migrations run on start
cd web && npm install && npm run dev              # app on http://localhost:5173
```

- Sign in with a test address such as `ana@example.test`. No email is sent in development: the login code is printed in the `./dev.sh` terminal.
- To try things between people, sign in as a second test person in a private window and add each other as friends (by @ or with the QR page).
- Pictures go to the local S3, never to the production bucket.

## Guidelines

- **Mobile first.** People use it at the bar or the restaurant, often on mobile data. Check 390 px width; nothing should scroll sideways. Buttons at least 44 px.
- **Portuguese UI** (pt-BR) for everything people see, including API error messages and `aria-label`s. Code, comments, commits and docs are in English.
- **Keep the look:** dark only, the lime-on-near-black "Roda" style. Use the CSS tokens in `web/src/app.css` (`--accent`, `--surface`, `--muted`, `--owe`…) and the icons in `web/src/lib/components/Icon.svelte`; no new colors or emoji in the UI.
- **Money is integer cents** everywhere (`*_cents` in the API and the database). Use `money()` from `web/src/lib/format.ts` to show it and the `MoneyInput` component to type it.
- **The receiver always gets exactly what they're owed.** Any payment fee (cards, automatic Pix, later) is added on top for the payer, never taken from the receiver.
- **Database changes** are new files in `api/migrations/` (`004_….sql`, `005_….sql`…). They are embedded in the API and applied in order when it starts. Never edit a migration that is already on `main`: production has run it.
- **Pictures** are shrunk in the browser (`web/src/lib/image.ts`) and checked again by the API (`api/image.go`). Keep both when adding a new upload.
- **Accessibility:** visible focus, labels on inputs, `aria-label` on icon-only buttons, and nothing communicated by color alone.

## Never commit secrets 🔒

- `.env` files are git-ignored. Keep it that way. `.env.example` only has placeholders.
- No real API keys, passwords, tokens, server addresses or anyone's email address in code, tests, screenshots, issues or PRs. Test people use `@example.test`.
- Production secrets live only in GitHub (environment `production`, used by `main` only) and on the server.
- If you leaked something by accident, tell Nelson right away. The key has to be rotated; deleting the commit isn't enough.
