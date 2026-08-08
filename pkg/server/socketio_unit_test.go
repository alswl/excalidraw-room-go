package server

import (
	"bytes"
	"testing"
)

type byteBuffer struct{ data []byte }

func (b byteBuffer) Bytes() []byte { return b.data }

func TestParseBroadcastArgs(t *testing.T) {
	tests := []struct {
		name string
		args []any
		ok   bool
	}{
		{
			name: "binary buffers",
			args: []any{"room-a", byteBuffer{[]byte{1, 2}}, bytes.NewBuffer([]byte{3, 4})},
			ok:   true,
		},
		{name: "missing arguments", args: []any{"room-a", []byte{1}}, ok: false},
		{name: "non-string room", args: []any{42, []byte{1}, []byte{2}}, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roomID, data, iv, ok := parseBroadcastArgs(tt.args)
			if ok != tt.ok {
				t.Fatalf("parseBroadcastArgs() ok = %v, want %v", ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if roomID != "room-a" || !bytes.Equal(data, []byte{1, 2}) || !bytes.Equal(iv, []byte{3, 4}) {
				t.Fatalf("parseBroadcastArgs() = (%q, %v, %v), want binary payload", roomID, data, iv)
			}
		})
	}
}

func TestBufferToBytes(t *testing.T) {
	if got := bufferToBytes(byteBuffer{[]byte{1, 2}}); !bytes.Equal(got, []byte{1, 2}) {
		t.Fatalf("Bytes buffer = %v, want [1 2]", got)
	}
	if got := bufferToBytes(bytes.NewBufferString("hello")); string(got) != "hello" {
		t.Fatalf("reader buffer = %q, want hello", got)
	}
	if got := bufferToBytes("not binary"); got != nil {
		t.Fatalf("unsupported value = %v, want nil", got)
	}
}

func TestParseFollowPayload(t *testing.T) {
	tests := []struct {
		name string
		args []any
		ok   bool
	}{
		{
			name: "typed payload",
			args: []any{userFollowPayload{UserToFollow: userToFollow{SocketID: "socket-a"}, Action: "FOLLOW"}},
			ok:   true,
		},
		{
			name: "decoded JSON object",
			args: []any{map[string]any{"userToFollow": map[string]any{"socketId": "socket-a"}, "action": "UNFOLLOW"}},
			ok:   true,
		},
		{name: "no payload", args: nil, ok: false},
		{name: "missing socket ID", args: []any{map[string]any{"action": "FOLLOW"}}, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, ok := parseFollowPayload(tt.args)
			if ok != tt.ok {
				t.Fatalf("parseFollowPayload() ok = %v, want %v", ok, tt.ok)
			}
			if tt.ok && payload.UserToFollow.SocketID != "socket-a" {
				t.Fatalf("socket ID = %q, want socket-a", payload.UserToFollow.SocketID)
			}
		})
	}
}
