//go:build !unix

package v0_2

// pollFd is a placeholder for non-Unix/Windows platforms.
type pollFd struct {
	Fd      int32
	Events  int16
	Revents int16
}

// Event constants are defined but will not be used in the fallback implementation.
const (
	pollEventRead  = 0x1
	pollEventWrite = 0x2
	pollEventError = 0x4
)

// poll provides a fallback implementation for platforms without native I/O multiplexing.
// It immediately returns 0, indicating no file descriptors are ready. This signals
// the main poll logic to use reflect.Select to wait on channel-based pollables,
// which is the correct behavior for timers and other non-FD events.
func poll(fds []pollFd, timeout int) (int, error) {
	// On unsupported platforms, we cannot poll file descriptors.
	// We return 0 immediately, which will cause the poll loop to
	// rely on the reflect.Select path for channel-based events.
	return 0, nil
}
