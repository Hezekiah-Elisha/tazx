//go:build unix

package libs

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func terminalSize(f *os.File) (int, int) {
	if f == nil {
		return 0, 0
	}
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws == nil {
		return 0, 0
	}
	return int(ws.Col), int(ws.Row)
}

func registerResize(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGWINCH)
}

func unregisterResize(ch chan<- os.Signal) {
	signal.Stop(ch)
}
