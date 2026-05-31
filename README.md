# the-verdict-paradox
Defend your identity.

## Deployment modes

### Local development

- Frontend: run `npm run dev` in `frontend/`
- Backend: run the Go service on `:8000`
- The frontend keeps using relative `/v1` and `/ws` paths, with Vite proxy forwarding them locally

### Single-service production

- Build and run the Go backend image from [backend/app/game/Dockerfile](/home/mikufan/code/the-verdict-paradox/backend/app/game/Dockerfile)
- That image now builds the frontend and serves the frontend static files, API, and WebSocket from the same origin
- `docker-compose.yml` exposes only the backend service on port `8000`

`GitHub Pages` cannot host this full stack because it only serves static files and cannot run the Go backend or WebSocket service.
