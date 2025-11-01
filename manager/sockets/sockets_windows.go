//go:build windows

package sockets

import "golang.org/x/sys/windows"

func CloseFd(fd int) {
	if fd != 0 {
		windows.Close(windows.Handle(fd))
	}
}
