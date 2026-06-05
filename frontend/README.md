# PulseFeed Frontend

React + TypeScript + Vite + Tailwind CSS frontend for end-to-end testing the Go backend.

## Run

```bash
npm install
npm run dev
```

By default the API client reads:

```bash
VITE_API_BASE_URL=http://localhost:8080
```

If `VITE_API_BASE_URL` is not set, the app falls back to `/api` and the Vite dev server proxies `/api/*` to `http://localhost:8080`.

## Implemented

- Login and register with `/account/login` and `/account/register`
- Token storage in `localStorage`
- Unified JSON API client with Bearer auth and 401 cleanup
- Video feeds: recommend, latest, following, popularity
- Recommendation detail hydration via `/video/listDetails`
- Real follow status via `/social/isFollowed`
- HTML5 video auto play/pause by viewport visibility
- Like and unlike with optimistic rollback
- Comments list, publish, delete, and report
- Notification list, unread count, mark read, and SSE stream
- Private messages with `peer_id`, `before_id`, and `limit`
- URL-based video publishing, direct file upload, and chunked MP4 upload
- Video/comment reporting
- Event tracking for impression, view, play_complete, and share

## Verify

```bash
npm test -- src/api/__tests__/pulsefeed.test.ts
npm run build
```

The Go backend allows local Vite origins through a small CORS middleware in `backend/internal/http/router.go`.
