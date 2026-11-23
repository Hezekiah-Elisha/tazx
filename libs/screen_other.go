//go:build !unix

package libs

import "os"

func terminalSize(_ *os.File) (int, int) {
	return 0, 0
}

func registerResize(_ chan<- os.Signal) {}

func unregisterResize(_ chan<- os.Signal) {}
