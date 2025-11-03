//go:build !unix && !windows

package sockets

func CloseFd(fd int) {}
