# Excalidraw Collaboration Server (Go)

A from-scratch, drop-in compatible reimplementation of the
[excalidraw-room](https://github.com/excalidraw/excalidraw-room) collaboration
server in Go. It speaks the exact same HTTP and Socket.IO wire protocol as the
original Node/Express server, so existing Excalidraw clients connect and
collaborate without any changes.

- **HTTP layer** — [huma v2](https://huma.rocks) (on a [chi](https://github.com/go-chi/chi) router)
- **Realtime layer** — [zishang520/socket.io](https://github.com/zishang520/socket.io), a faithful Go port of the official Socket.IO v4 server

## Features

- Identical API surface to the original server (see [API](#api))
- WebSocket **and** HTTP long-polling transports, with automatic upgrade
- Binary relay (`ArrayBuffer` / `Uint8Array`) for Excalidraw's encrypted scene data
- Follow mode (`follow@{socketId}` rooms) with follower change notifications
- Engine.IO v3 *and* v4 clients (`allowEIO3`)
- CORS configurable per the original server
- Static file serving from `./public`
- Graceful shutdown on `SIGINT` / `SIGTERM`
- Fully Go-native build, runtime, and test toolchain — no Node.js or PM2

## Architecture

```
                 ┌────────────────────────────────────────────┐
 browser /       │  excalidraw-room-go                         │
 Excalidraw      │                                            │
 client          │   /socket.io/* ──► zishang520/socket.io     │
  ─────────────► │       (Engine.IO v3/v4, polling + ws)      │
                 │              │ room events / broadcasts    │
                 │              ▼                            │
                 │   GET /  ───► huma (chi)                   │
                 │   /*    ───► static files from ./public    │
                 └────────────────────────────────────────────┘
```

The two layers share one HTTP server. Requests under `/socket.io/` are handled
by the Socket.IO engine; everything else is served by the huma router. Both are
mounted in [`buildRouter`](main.go), and the server runs under a single
`http.Server` with graceful shutdown.

## API

The API is byte-for-byte compatible with the original server.

### HTTP

| Method | Path      | Response                                                            |
| ------ | --------- | ------------------------------------------------------------------- |
| GET    | `/`       | `Excalidraw collaboration server is up :)` (`text/html; charset=utf-8`) |
| GET    | `/*`      | Static files from `./public` (mirrors `express.static("public")`)        |

### Socket.IO (served at `/socket.io/`)

Events received from clients:

| Event                      | Payload                                                    | Behavior                                                                 |
| -------------------------- | ---------------------------------------------------------- | ------------------------------------------------------------------------ |
| `join-room`                | `roomID: string`                                           | joins the room; emits `first-in-room` (solo) or `new-user` + `room-user-change` |
| `server-broadcast`         | `(roomID, encryptedData: ArrayBuffer, iv: Uint8Array)`     | relays `client-broadcast` to everyone in the room except the sender       |
| `server-volatile-broadcast`| `(roomID, encryptedData: ArrayBuffer, iv: Uint8Array)`     | same, but volatile (skipped when the transport isn't writable)            |
| `user-follow`              | `{ userToFollow: { socketId, username }, action: "FOLLOW"\|"UNFOLLOW" }` | tracks `follow@{socketId}` rooms and notifies the followed user via `user-follow-room-change` |

Events emitted to clients:

`init-room`, `first-in-room`, `new-user`, `room-user-change`,
`client-broadcast`, `user-follow-room-change`, `broadcast-unfollow`.

Connection lifecycle replicates the original, including the `disconnecting`
handler that broadcasts `room-user-change` to the remaining room members and
emits `broadcast-unfollow` when a follow room becomes empty.

### Configuration

Configuration is read from environment variables (12-factor style) with
Go-friendly defaults:

| Env var       | Default | Notes                    |
| ------------- | ------- | ------------------------ |
| `PORT`        | `8080`  | HTTP listen port         |
| `CORS_ORIGIN` | `*`     | allowed Socket.IO origin |

## Running

Requires Go 1.26+ (see the `go` directive in `go.mod`).

### Development

```sh
# run the server on :8080
go run .
```

### Production

```sh
go build -o excalidraw-room-server .
PORT=8080 ./excalidraw-room-server
```

### Docker

```sh
docker build -t excalidraw-room-go .
docker run --rm -p 8080:8080 excalidraw-room-go
```

The server exits with code `0` after a graceful shutdown (`SIGINT`/`SIGTERM`):
socket.io clients are disconnected first so long-polling requests drain, then
in-flight HTTP requests get up to 10 seconds to finish.

## Testing

`main_test.go` is a Go integration test that boots the full handler and drives
it with [socket.io-client-go](https://github.com/zishang520/socket.io-client-go)
— a Go port of the real Socket.IO v4 client. It covers the HTTP endpoints,
every Socket.IO event, follow mode, disconnecting, and CORS preflight:

```sh
go test ./...
```

The behavior was validated side-by-side against the original Node server during
development: the same scenario suite passes on both implementations.

## Compatibility with the original excalidraw-room

The protocol surface — HTTP responses, Socket.IO event names, and payload
shapes — is identical to the original. (Configuration is Go-standard
environment variables rather than the original's `NODE_ENV`-based setup.)

A few notes:

- **Room events are serialized.** The original ran on a single-threaded event
  loop, so its handlers never interleaved. The Go adapter dispatches each
  socket's events on its own goroutine, so `join-room`, `user-follow`, and
  `disconnecting` are processed under a shared mutex to reproduce the original's
  ordering and avoid dropping membership updates.
- **`/socket.io/socket.io.js` is not served.** Excalidraw bundles its own
  Socket.IO client, so the convenience asset endpoint is omitted.
- **HTTP headers differ cosmetically.** The `Content-Type` and body of `GET /`
  are byte-identical, but Express-specific headers (`X-Powered-By`, `ETag`,
  keep-alive hints) are not reproduced.
- **Engine.IO open-packet JSON field order and session-id format** differ (both
  are opaque to clients).

## License

MIT
