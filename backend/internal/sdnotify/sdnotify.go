// Package sdnotify speaks systemd's notify protocol (sd_notify(3)) without a
// dependency: READY=1 once the listener is up and WATCHDOG=1 while the
// process is healthy. Outside systemd (no NOTIFY_SOCKET) everything is a no-op.
package sdnotify

import (
	"context"
	"net"
	"os"
	"strconv"
	"time"
)

// Notify sends one state line ("READY=1", "WATCHDOG=1", "STOPPING=1").
func Notify(state string) error {
	sock := os.Getenv("NOTIFY_SOCKET")
	if sock == "" {
		return nil
	}
	if sock[0] == '@' { // abstract socket
		sock = "\x00" + sock[1:]
	}
	c, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: sock, Net: "unixgram"})
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = c.Write([]byte(state))
	return err
}

// Watchdog pings systemd at half the WatchdogSec interval while healthy()
// holds; a hung database or deadlock lets systemd restart the service.
func Watchdog(ctx context.Context, healthy func(context.Context) bool) {
	usec, err := strconv.ParseInt(os.Getenv("WATCHDOG_USEC"), 10, 64)
	if err != nil || usec <= 0 || os.Getenv("NOTIFY_SOCKET") == "" {
		return
	}
	every := time.Duration(usec) * time.Microsecond / 2
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			hctx, cancel := context.WithTimeout(ctx, every/2)
			ok := healthy(hctx)
			cancel()
			if ok {
				Notify("WATCHDOG=1")
			}
		}
	}
}
