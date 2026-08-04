package main

import (
	"bytes"
	"io"
	"strings"

	"github.com/zishang520/engine.io/v2/transports"
	"github.com/zishang520/engine.io/v2/types"
	socketio "github.com/zishang520/socket.io/v2/socket"
)

const followRoomPrefix = "follow@"

// setupSocketIO builds the socket.io server with behavior identical to the
// original excalidraw-room Node server:
//
//   - transports: websocket + polling (websocket preferred, upgrades allowed)
//   - allowEIO3: true
//   - cors: {
//     origin: CORS_ORIGIN || "*",
//     allowedHeaders: ["Content-Type", "Authorization"],
//     credentials: true,
//     }
//   - pingInterval: 25000ms, pingTimeout: 20000ms (socket.io defaults)
//
// The returned server is an http.Handler served at /socket.io/.
func setupSocketIO(corsOrigin string) *socketio.Server {
	opts := socketio.DefaultServerOptions()
	opts.SetPath("/socket.io")
	opts.SetServeClient(false)
	opts.SetTransports(types.NewSet(transports.WEBSOCKET, transports.POLLING))
	opts.SetAllowUpgrades(true)
	opts.SetAllowEIO3(true)
	opts.SetCors(&types.Cors{
		Origin:         corsOrigin,
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		Credentials:    true,
	})

	io := socketio.NewServer(nil, opts)

	io.On("connection", func(clients ...any) {
		s, ok := clients[0].(*socketio.Socket)
		if !ok {
			return
		}
		handleSocketConnection(io, s)
	})

	return io
}

// handleSocketConnection wires up all the per-socket event handlers, matching
// the original server's connection handler.
func handleSocketConnection(io *socketio.Server, s *socketio.Socket) {
	// io.to(`${socket.id}`).emit("init-room")
	s.Emit("init-room")

	s.On("join-room", func(args ...any) {
		roomID, ok := args[0].(string)
		if !ok || roomID == "" {
			return
		}
		s.Join(socketio.Room(roomID))

		io.In(socketio.Room(roomID)).FetchSockets()(func(sockets []*socketio.RemoteSocket, _ error) {
			ids := socketIDs(sockets)
			if len(ids) <= 1 {
				s.Emit("first-in-room")
			} else {
				// socket.broadcast.to(roomID).emit("new-user", socket.id)
				s.Broadcast().To(socketio.Room(roomID)).Emit("new-user", s.Id())
			}
			// io.in(roomID).emit("room-user-change", ids)
			io.In(socketio.Room(roomID)).Emit("room-user-change", ids)
		})
	})

	s.On("server-broadcast", func(args ...any) {
		roomID, data, iv, ok := parseBroadcastArgs(args)
		if !ok {
			return
		}
		// socket.broadcast.to(roomID).emit("client-broadcast", encryptedData, iv)
		s.Broadcast().To(socketio.Room(roomID)).Emit("client-broadcast", data, iv)
	})

	s.On("server-volatile-broadcast", func(args ...any) {
		roomID, data, iv, ok := parseBroadcastArgs(args)
		if !ok {
			return
		}
		// socket.volatile.broadcast.to(roomID).emit("client-broadcast", encryptedData, iv)
		s.Volatile().Broadcast().To(socketio.Room(roomID)).Emit("client-broadcast", data, iv)
	})

	s.On("user-follow", func(args ...any) {
		payload, ok := parseFollowPayload(args)
		if !ok {
			return
		}
		roomID := socketio.Room(followRoomPrefix + payload.UserToFollow.SocketID)
		switch payload.Action {
		case "FOLLOW":
			s.Join(roomID)
		case "UNFOLLOW":
			s.Leave(roomID)
		default:
			return
		}

		io.In(roomID).FetchSockets()(func(sockets []*socketio.RemoteSocket, _ error) {
			followedBy := socketIDs(sockets)
			// io.to(payload.userToFollow.socketId).emit("user-follow-room-change", followedBy)
			io.To(socketio.Room(payload.UserToFollow.SocketID)).Emit("user-follow-room-change", followedBy)
		})
	})

	s.On("disconnecting", func(args ...any) {
		for _, room := range s.Rooms().Keys() {
			roomName := string(room)
			io.In(room).FetchSockets()(func(sockets []*socketio.RemoteSocket, _ error) {
				others := make([]*socketio.RemoteSocket, 0, len(sockets))
				for _, sock := range sockets {
					if sock.Id() != s.Id() {
						others = append(others, sock)
					}
				}
				otherIDs := socketIDs(others)
				isFollowRoom := strings.HasPrefix(roomName, followRoomPrefix)

				if !isFollowRoom && len(others) > 0 {
					// socket.broadcast.to(roomID).emit("room-user-change", otherClients.map(s => s.id))
					s.Broadcast().To(room).Emit("room-user-change", otherIDs)
				}

				if isFollowRoom && len(others) == 0 {
					// io.to(followedSocketId).emit("broadcast-unfollow")
					followedID := strings.TrimPrefix(roomName, followRoomPrefix)
					io.To(socketio.Room(followedID)).Emit("broadcast-unfollow")
				}
			})
		}
	})
}

// socketIDs extracts the ordered socket ids from fetched remote sockets.
func socketIDs(sockets []*socketio.RemoteSocket) []socketio.SocketId {
	ids := make([]socketio.SocketId, 0, len(sockets))
	for _, s := range sockets {
		ids = append(ids, s.Id())
	}
	return ids
}

// parseBroadcastArgs extracts (roomID, encryptedData, iv) from a
// server-broadcast / server-volatile-broadcast packet.
func parseBroadcastArgs(args []any) (roomID string, data, iv []byte, ok bool) {
	if len(args) < 3 {
		return "", nil, nil, false
	}
	roomID, ok = args[0].(string)
	if !ok {
		return "", nil, nil, false
	}
	return roomID, bufferToBytes(args[1]), bufferToBytes(args[2]), true
}

// bufferToBytes extracts raw bytes from a decoded binary attachment. The
// socket.io parser reconstructs ArrayBuffer / Uint8Array values as
// *types.BytesBuffer; []byte and io.Reader are also accepted for robustness.
func bufferToBytes(v any) []byte {
	switch b := v.(type) {
	case []byte:
		return b
	case string:
		return []byte(b)
	case *bytes.Buffer:
		return b.Bytes()
	case io.Reader:
		data, _ := io.ReadAll(b)
		return data
	default:
		if br, ok := v.(interface{ Bytes() []byte }); ok {
			return br.Bytes()
		}
		return nil
	}
}

type userToFollow struct {
	SocketID string `json:"socketId"`
	Username string `json:"username"`
}

type userFollowPayload struct {
	UserToFollow userToFollow `json:"userToFollow"`
	Action       string       `json:"action"`
}

// parseFollowPayload extracts the user-follow payload from a decoded packet.
func parseFollowPayload(args []any) (userFollowPayload, bool) {
	if len(args) < 1 {
		return userFollowPayload{}, false
	}
	m, ok := args[0].(map[string]any)
	if !ok {
		return userFollowPayload{}, false
	}
	uto, ok := m["userToFollow"].(map[string]any)
	if !ok {
		return userFollowPayload{}, false
	}
	socketID, _ := uto["socketId"].(string)
	username, _ := uto["username"].(string)
	action, _ := m["action"].(string)
	if socketID == "" {
		return userFollowPayload{}, false
	}
	return userFollowPayload{
		UserToFollow: userToFollow{SocketID: socketID, Username: username},
		Action:       action,
	}, true
}
