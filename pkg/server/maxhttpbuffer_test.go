package server

import (
	"bytes"
	"testing"
	"time"
)

// TestLargeBroadcastDeliveredWithinBuffer guards against
// alswl/excalidraw-collaboration#73: socket.io's default maxHttpBufferSize is
// 1MB, and Excalidraw broadcasts the whole (encrypted) scene on join and
// periodic full-sync. With many strokes the scene exceeds 1MB, the server
// drops the sender's connection and peers stop updating (white screens /
// lost elements). A 2MB broadcast must be delivered, not dropped.
func TestLargeBroadcastDeliveredWithinBuffer(t *testing.T) {
	ts := startServer(t)

	aRec := &clientRecorder{}
	a := newTestSocket(t, ts.URL, aRec)
	defer a.Disconnect()
	bRec := &clientRecorder{}
	b := newTestSocket(t, ts.URL, bRec)
	defer b.Disconnect()

	a.Emit("join-room", "roomA")
	waitFor(t, 3*time.Second, func() bool { return len(aRec.getRoomChange()) >= 1 }, "A joined room")
	b.Emit("join-room", "roomA")
	waitFor(t, 3*time.Second, func() bool { return len(bRec.getRoomChange()) == 2 }, "both in room")

	// 2MB > socket.io's 1MB default; must still be relayed to the peer.
	big := bytes.Repeat([]byte{0xAB}, 2*1024*1024)
	iv := []byte{9, 8, 7}
	a.Emit("server-broadcast", "roomA", big, iv)

	waitFor(t, 5*time.Second, func() bool {
		d, _ := bRec.getClientBcast()
		return bytes.Equal(d, big)
	}, "B receives 2MB client-broadcast")
}
