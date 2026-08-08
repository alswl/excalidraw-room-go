package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alswl/excalidraw-room-go/pkg/config"
	"github.com/zishang520/engine.io-client-go/transports"
	"github.com/zishang520/engine.io/v2/types"
	client "github.com/zishang520/socket.io-client-go/socket"
)

// startServer boots the full application handler on an ephemeral port.
func startServer(t *testing.T) *httptest.Server {
	t.Helper()
	app := New(config.Config{CORSOrigin: "*", StaticDir: "../../public"})
	ts := httptest.NewServer(app.Handler)
	t.Cleanup(func() {
		ts.Close()
		if err := app.Close(); err != nil {
			t.Errorf("close socket.io: %v", err)
		}
	})
	return ts
}

// clientRecorder captures the events a test socket receives.
type clientRecorder struct {
	mu                sync.Mutex
	initRoom          int
	firstInRoom       int
	newUser           string
	roomChange        []string
	clientBcastData   []byte
	clientBcastIV     []byte
	followChange      []string
	broadcastUnfollow int
}

func (r *clientRecorder) getInitRoom() int    { r.mu.Lock(); defer r.mu.Unlock(); return r.initRoom }
func (r *clientRecorder) getFirstInRoom() int { r.mu.Lock(); defer r.mu.Unlock(); return r.firstInRoom }
func (r *clientRecorder) getNewUser() string  { r.mu.Lock(); defer r.mu.Unlock(); return r.newUser }
func (r *clientRecorder) getRoomChange() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.roomChange...)
}
func (r *clientRecorder) getClientBcast() ([]byte, []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clientBcastData, r.clientBcastIV
}
func (r *clientRecorder) clearClientBcast() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clientBcastData = nil
	r.clientBcastIV = nil
}
func (r *clientRecorder) getFollowChange() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.followChange...)
}
func (r *clientRecorder) getBroadcastUnfollow() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.broadcastUnfollow
}

// newTestSocket connects a socket.io client and wires it up to the recorder.
func newTestSocket(t *testing.T, url string, rec *clientRecorder) *client.Socket {
	t.Helper()
	opts := client.DefaultOptions()
	opts.SetReconnection(false)
	opts.SetTransports(types.NewSet(transports.Polling, transports.WebSocket))

	m := client.NewManager(url, opts)
	s := m.Socket("/", opts)

	s.On("init-room", func(args ...any) { rec.mu.Lock(); rec.initRoom++; rec.mu.Unlock() })
	s.On("first-in-room", func(args ...any) { rec.mu.Lock(); rec.firstInRoom++; rec.mu.Unlock() })
	s.On("new-user", func(args ...any) { rec.mu.Lock(); rec.newUser = toString(args[0]); rec.mu.Unlock() })
	s.On("room-user-change", func(args ...any) { rec.mu.Lock(); rec.roomChange = toStrings(args[0]); rec.mu.Unlock() })
	s.On("client-broadcast", func(args ...any) {
		rec.mu.Lock()
		rec.clientBcastData = bufToBytes(args[0])
		rec.clientBcastIV = bufToBytes(args[1])
		rec.mu.Unlock()
	})
	s.On("user-follow-room-change", func(args ...any) { rec.mu.Lock(); rec.followChange = toStrings(args[0]); rec.mu.Unlock() })
	s.On("broadcast-unfollow", func(args ...any) { rec.mu.Lock(); rec.broadcastUnfollow++; rec.mu.Unlock() })

	connected := make(chan struct{})
	s.On("connect", func(args ...any) { close(connected) })
	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatalf("socket did not connect to %s", url)
	}
	return s
}

func TestHTTPEndpoints(t *testing.T) {
	ts := startServer(t)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != rootMessage {
		t.Errorf("GET / body = %q, want %q", body, rootMessage)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("GET / Content-Type = %q, want text/html; charset=utf-8", ct)
	}

	resp, err = http.Get(ts.URL + "/test64.png")
	if err != nil {
		t.Fatalf("GET /test64.png: %v", err)
	}
	png, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.HasPrefix(png, []byte{0x89, 'P', 'N', 'G'}) {
		t.Errorf("static file not served: status=%d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/nope")
	if err != nil {
		t.Fatalf("GET /nope: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /nope status = %d, want 404", resp.StatusCode)
	}

	for _, path := range []string{"/health", "/ready"} {
		resp, err = http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("GET %s status = %d, want 204", path, resp.StatusCode)
		}
	}
}

func TestSocketIORooms(t *testing.T) {
	ts := startServer(t)

	aRec := &clientRecorder{}
	a := newTestSocket(t, ts.URL, aRec)
	defer a.Disconnect()
	if a.Id() == "" {
		t.Fatal("socket id is empty")
	}
	waitFor(t, 3*time.Second, func() bool { return aRec.getInitRoom() == 1 }, "init-room")

	// First client in a room: first-in-room + room-user-change [self].
	a.Emit("join-room", "roomA")
	waitFor(t, 3*time.Second, func() bool { return aRec.getFirstInRoom() == 1 }, "first-in-room")
	waitFor(t, 3*time.Second, func() bool {
		ids := aRec.getRoomChange()
		return len(ids) == 1 && ids[0] == a.Id()
	}, "solo room-user-change")

	// Second client joins: A gets new-user + room-user-change, B gets neither.
	bRec := &clientRecorder{}
	b := newTestSocket(t, ts.URL, bRec)
	defer b.Disconnect()
	b.Emit("join-room", "roomA")

	waitFor(t, 3*time.Second, func() bool { return aRec.getNewUser() == b.Id() }, "A receives new-user")
	waitFor(t, 3*time.Second, func() bool {
		ids := aRec.getRoomChange()
		return len(ids) == 2 && contains(ids, a.Id()) && contains(ids, b.Id())
	}, "A room-user-change with both")
	waitFor(t, 3*time.Second, func() bool {
		ids := bRec.getRoomChange()
		return len(ids) == 2 && contains(ids, a.Id()) && contains(ids, b.Id())
	}, "B room-user-change with both")

	if bRec.getFirstInRoom() != 0 {
		t.Error("second client must not receive first-in-room")
	}
	if bRec.getNewUser() != "" {
		t.Error("second client must not receive new-user")
	}
}

func TestSocketIOBroadcast(t *testing.T) {
	ts := startServer(t)

	aRec := &clientRecorder{}
	a := newTestSocket(t, ts.URL, aRec)
	defer a.Disconnect()
	bRec := &clientRecorder{}
	b := newTestSocket(t, ts.URL, bRec)
	defer b.Disconnect()

	// Serialize the joins: the real excalidraw clients join at human-spaced
	// times, and simultaneous joins can race in the adapter.
	a.Emit("join-room", "roomA")
	waitFor(t, 3*time.Second, func() bool { return len(aRec.getRoomChange()) >= 1 }, "A joined room")
	b.Emit("join-room", "roomA")
	waitFor(t, 3*time.Second, func() bool { return len(bRec.getRoomChange()) == 2 }, "both in room")

	data := []byte{1, 2, 3, 4, 250}
	iv := []byte{9, 8, 7}
	a.Emit("server-broadcast", "roomA", data, iv)
	waitFor(t, 3*time.Second, func() bool {
		d, i := bRec.getClientBcast()
		return bytes.Equal(d, data) && bytes.Equal(i, iv)
	}, "B receives client-broadcast")
	if d, _ := aRec.getClientBcast(); d != nil {
		t.Error("sender must not receive its own broadcast")
	}

	// Volatile broadcast.
	bRec.clearClientBcast()
	a.Emit("server-volatile-broadcast", "roomA", []byte{255, 254}, iv)
	waitFor(t, 3*time.Second, func() bool {
		d, _ := bRec.getClientBcast()
		return bytes.Equal(d, []byte{255, 254})
	}, "B receives volatile client-broadcast")
}

func TestSocketIOFollow(t *testing.T) {
	ts := startServer(t)

	aRec := &clientRecorder{}
	a := newTestSocket(t, ts.URL, aRec)
	defer a.Disconnect()
	bRec := &clientRecorder{}
	b := newTestSocket(t, ts.URL, bRec)
	defer b.Disconnect()

	// A follows itself.
	a.Emit("user-follow", userFollowPayload{
		UserToFollow: userToFollow{SocketID: a.Id()},
		Action:       "FOLLOW",
	})
	waitFor(t, 3*time.Second, func() bool {
		ids := aRec.getFollowChange()
		return len(ids) == 1 && ids[0] == a.Id()
	}, "A follows A")

	// B follows A.
	b.Emit("user-follow", userFollowPayload{
		UserToFollow: userToFollow{SocketID: a.Id()},
		Action:       "FOLLOW",
	})
	waitFor(t, 3*time.Second, func() bool {
		ids := aRec.getFollowChange()
		return len(ids) == 2 && contains(ids, a.Id()) && contains(ids, b.Id())
	}, "B follows A")

	// B unfollows A.
	b.Emit("user-follow", userFollowPayload{
		UserToFollow: userToFollow{SocketID: a.Id()},
		Action:       "UNFOLLOW",
	})
	waitFor(t, 3*time.Second, func() bool {
		ids := aRec.getFollowChange()
		return len(ids) == 1 && ids[0] == a.Id()
	}, "B unfollows A")
}

func TestSocketIODisconnect(t *testing.T) {
	ts := startServer(t)

	aRec := &clientRecorder{}
	a := newTestSocket(t, ts.URL, aRec)
	bRec := &clientRecorder{}
	b := newTestSocket(t, ts.URL, bRec)
	defer b.Disconnect()

	a.Emit("join-room", "roomA")
	waitFor(t, 3*time.Second, func() bool { return len(aRec.getRoomChange()) >= 1 }, "A joined room")
	b.Emit("join-room", "roomA")
	waitFor(t, 3*time.Second, func() bool { return len(bRec.getRoomChange()) == 2 }, "both in room")

	// A follows B so follow@{B} becomes empty when A disconnects.
	a.Emit("user-follow", userFollowPayload{
		UserToFollow: userToFollow{SocketID: b.Id()},
		Action:       "FOLLOW",
	})
	waitFor(t, 3*time.Second, func() bool {
		ids := bRec.getFollowChange()
		return len(ids) == 1 && ids[0] == a.Id()
	}, "B notified of follower A")

	a.Disconnect()

	waitFor(t, 3*time.Second, func() bool {
		ids := bRec.getRoomChange()
		return len(ids) == 1 && ids[0] == b.Id()
	}, "B sees A leave room")
	waitFor(t, 3*time.Second, func() bool { return bRec.getBroadcastUnfollow() == 1 }, "B receives broadcast-unfollow")
}

func TestCORSPreflight(t *testing.T) {
	ts := startServer(t)

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/socket.io/?EIO=4&transport=polling", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Errorf("preflight status = %d, want 2xx", resp.StatusCode)
	}
	if ao := resp.Header.Get("Access-Control-Allow-Origin"); ao != "http://example.com" && ao != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q", ao)
	}
	if resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Access-Control-Allow-Credentials missing")
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", msg)
}

func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func toStrings(v any) []string {
	if arr, ok := v.([]any); ok {
		out := make([]string, len(arr))
		for i, a := range arr {
			out[i] = toString(a)
		}
		return out
	}
	return nil
}

func bufToBytes(v any) []byte {
	if b, ok := v.(interface{ Bytes() []byte }); ok {
		return b.Bytes()
	}
	if r, ok := v.(io.Reader); ok {
		data, _ := io.ReadAll(r)
		return data
	}
	return nil
}
