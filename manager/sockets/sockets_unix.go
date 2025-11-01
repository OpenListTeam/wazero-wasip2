//go:build unix

package sockets

import "golang.org/x/sys/unix"

func CloseFd(fd int) {
	unix.Close(fd)
}
