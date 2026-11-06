package sdnotify

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestNotifyAndWatchdog(t *testing.T) {
	if err := Notify("READY=1"); err != nil { // no NOTIFY_SOCKET: no-op
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "n.sock")
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	t.Setenv("NOTIFY_SOCKET", path)
	t.Setenv("WATCHDOG_USEC", "40000") // ping every 20 ms
	if err := Notify("READY=1"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Watchdog(ctx, func(context.Context) bool { return true })
	buf := make([]byte, 64)
	got := []string{}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for len(got) < 2 {
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, string(buf[:n]))
	}
	if got[0] != "READY=1" || got[1] != "WATCHDOG=1" {
		t.Fatalf("got %v", got)
	}
}
