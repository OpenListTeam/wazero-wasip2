//go:build unix

package v0_2

import "golang.org/x/sys/unix"

type pollFd = unix.PollFd

const (
	pollEventRead  = unix.POLLIN | unix.POLLPRI
	pollEventWrite = unix.POLLOUT
	pollEventError = unix.POLLERR | unix.POLLHUP | unix.POLLNVAL
)

func poll(fds []pollFd, timeout int) (int, error) {
	return unix.Poll(fds, timeout)
}
