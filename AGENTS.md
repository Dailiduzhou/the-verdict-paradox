# the-verdict-paradox (agent notes)

## Where the app lives
- The React app is in `frontend/` (Vite + React + TypeScript). Repo root is mostly empty.

## Commands (run in `frontend/`)
- Install: `npm install`
- Dev: `npm run dev`
- Lint: `npm run lint`
- Build (typecheck + bundle): `npm run build`
- Preview build: `npm run preview`

## Runtime config (non-obvious)
- API base URL is required via `VITE_API_BASE_URL`.
  Use `frontend/.env.example` as a template for `frontend/.env`.

## App flow / entrypoints
- Entry: `frontend/src/main.tsx` (MUI theme + `CssBaseline`).
- UI state machine is in `frontend/src/App.tsx`: `splash -> checking -> auth -> game`.
- “Press any key to enter” also supports pointer/touch (`keydown` + `pointerdown`).

## Auth/token behavior (easy to break)
- Token is stored in `localStorage` under key `token`.
- Verification: `POST /token/verify` with body `{ token }` expecting `{ valid: boolean }`.
- Login: `POST /auth/login` body `{ username, password }` expecting `{ token }` or `{ msg }`.
- Register: `POST /auth/register` body `{ username, password }` expecting `{ token }` or `{ msg }`.
- API helper: `frontend/src/api.ts` uses `fetch` and throws `Error(msg)` when response is non-2xx.
