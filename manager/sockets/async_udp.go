package sockets

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/OpenListTeam/wazero-wasip2/common/bytespool"
	manager_io "github.com/OpenListTeam/wazero-wasip2/manager/io"
)

const defaultUDPBufferSize = 256 // 默认缓冲 256 个数据报

// --- Asynchronous UDP Reader ---

type AsyncUDPReader struct {
	conn   *net.UDPConn
	buffer []IncomingDatagram
	mutex  sync.Mutex
	ready  *manager_io.ChannelPollable
	done   chan struct{}
	err    error
	once   sync.Once
}

func NewAsyncUDPReader(conn *net.UDPConn) *AsyncUDPReader {
	wrapper := &AsyncUDPReader{
		conn:   conn,
		buffer: make([]IncomingDatagram, 0, 32),
		ready:  manager_io.NewPollable(nil),
		done:   make(chan struct{}),
	}
	go wrapper.run()
	return wrapper
}

func (ar *AsyncUDPReader) run() {
	buf := bytespool.Alloc(64 * 1024)
	defer bytespool.Free(buf)
	for {
		select {
		case <-ar.done:
			return
		default:
		}

		n, remoteAddr, err := ar.conn.ReadFromUDP(buf)

		ar.mutex.Lock()
		if err != nil {
			select {
			case <-ar.done:
				ar.mutex.Unlock()
				return
			default:
			}
			ar.err = err
			ar.ready.SetReady()
			ar.mutex.Unlock()
			return
		}

		if n > 0 {
			wasEmpty := len(ar.buffer) == 0
			data := make([]byte, n)
			copy(data, buf[:n])

			wasiAddr, factoryErr := ToIPSocketAddress(remoteAddr)
			if factoryErr == nil {
				ar.buffer = append(ar.buffer, IncomingDatagram{
					Data:          data,
					RemoteAddress: wasiAddr,
				})
				if wasEmpty {
					ar.ready.SetReady()
				}
			}
		}
		ar.mutex.Unlock()
	}
}

func (ar *AsyncUDPReader) Receive(maxResults uint64) ([]IncomingDatagram, error) {
	ar.mutex.Lock()
	defer ar.mutex.Unlock()

	if len(ar.buffer) > 0 {
		count := uint64(len(ar.buffer))
		if count > maxResults {
			count = maxResults
		}

		datagrams := make([]IncomingDatagram, count)
		copy(datagrams, ar.buffer[:count])
		ar.buffer = ar.buffer[count:]

		if len(ar.buffer) == 0 && ar.err == nil {
			ar.ready.Reset()
		}
		return datagrams, nil
	}
	if ar.err != nil && ar.err != io.EOF {
		return nil, ar.err
	}
	return nil, nil // Buffer is empty and no permanent error
}

func (ar *AsyncUDPReader) Subscribe() manager_io.IPollable {
	ar.mutex.Lock()
	defer ar.mutex.Unlock()
	if len(ar.buffer) > 0 || ar.err != nil {
		ar.ready.SetReady()
	}
	return ar.ready
}

func (ar *AsyncUDPReader) Close() {
	ar.once.Do(func() {
		close(ar.done)
	})
}

// --- Asynchronous UDP Writer with Backpressure ---

type AsyncUDPWriter struct {
	conn          *net.UDPConn
	buffer        []OutgoingDatagram
	mutex         sync.Mutex
	cond          *sync.Cond
	ready         *manager_io.ChannelPollable
	done          chan struct{}
	err           error
	maxBufferSize int
	once          sync.Once
}

func NewAsyncUDPWriter(conn *net.UDPConn) *AsyncUDPWriter {
	wrapper := &AsyncUDPWriter{
		conn:          conn,
		buffer:        make([]OutgoingDatagram, 0, defaultUDPBufferSize),
		ready:         manager_io.NewPollable(nil),
		done:          make(chan struct{}),
		maxBufferSize: defaultUDPBufferSize,
	}
	wrapper.cond = sync.NewCond(&wrapper.mutex)
	wrapper.ready.SetReady()
	go wrapper.run()
	return wrapper
}

func (aw *AsyncUDPWriter) run() {
	aw.mutex.Lock()
	defer aw.mutex.Unlock()

	for {
		for len(aw.buffer) == 0 {
			select {
			case <-aw.done:
				return
			default:
				aw.cond.Wait()
			}
		}

		datagramsToSend := make([]OutgoingDatagram, len(aw.buffer))
		copy(datagramsToSend, aw.buffer)
		aw.buffer = aw.buffer[:0]

		aw.mutex.Unlock()

		var writeErr error
		for _, dg := range datagramsToSend {
			var remoteAddr *net.UDPAddr
			if dg.RemoteAddress.Some != nil {
				remoteAddr, writeErr = FromIPSocketAddressToUDPAddr(*dg.RemoteAddress.Some)
				if writeErr != nil {
					break
				}
			}

			if remoteAddr != nil {
				_, writeErr = aw.conn.WriteToUDP(dg.Data, remoteAddr)
			} else {
				_, writeErr = aw.conn.Write(dg.Data)
			}
			if writeErr != nil {
				break
			}
		}

		aw.mutex.Lock()

		if writeErr != nil {
			aw.err = writeErr
		}
		aw.ready.SetReady()
	}
}

func (aw *AsyncUDPWriter) Send(datagrams []OutgoingDatagram) (uint64, error) {
	aw.mutex.Lock()
	defer aw.mutex.Unlock()

	if aw.err != nil {
		return 0, aw.err
	}

	available := aw.maxBufferSize - len(aw.buffer)
	if available <= 0 {
		return 0, nil
	}

	count := uint64(len(datagrams))
	if count > uint64(available) {
		count = uint64(available)
	}

	aw.buffer = append(aw.buffer, datagrams[:count]...)

	if count > 0 {
		aw.cond.Signal()
	}

	if len(aw.buffer) >= aw.maxBufferSize {
		aw.ready.Reset()
	}

	return count, nil
}

// AvailableSpace 返回缓冲区中可用于写入的数据报数量。
func (aw *AsyncUDPWriter) AvailableSpace() uint64 {
	aw.mutex.Lock()
	defer aw.mutex.Unlock()
	return uint64(aw.maxBufferSize - len(aw.buffer))
}

func (aw *AsyncUDPWriter) Subscribe() manager_io.IPollable {
	aw.mutex.Lock()
	defer aw.mutex.Unlock()
	if len(aw.buffer) < aw.maxBufferSize || aw.err != nil {
		aw.ready.SetReady()
	}
	return aw.ready
}

func (aw *AsyncUDPWriter) Close() {
	aw.once.Do(func() {
		aw.mutex.Lock()
		close(aw.done)
		aw.cond.Broadcast()
		aw.mutex.Unlock()
	})
}

// fromIPSocketAddressToUDPAddr 将 WIT 的 IPSocketAddress 转换为 Go 的 *net.UDPAddr。
func FromIPSocketAddressToUDPAddr(addr IPSocketAddress) (*net.UDPAddr, error) {
	if addr.IPV4 != nil {
		ip := net.IP(addr.IPV4.Address[:])
		return &net.UDPAddr{IP: ip, Port: int(addr.IPV4.Port)}, nil
	}
	if addr.IPV6 != nil {
		ip := make(net.IP, 16)
		for i, part := range addr.IPV6.Address {
			binary.BigEndian.PutUint16(ip[i*2:], part)
		}
		return &net.UDPAddr{IP: ip, Port: int(addr.IPV6.Port), Zone: ""}, nil
	}
	return nil, errors.New("invalid ip-socket-address")
}

func ToIPSocketAddress(addr net.Addr) (IPSocketAddress, error) {
	switch tcpAddr := addr.(type) {
	case *net.TCPAddr:

		if ipv4 := tcpAddr.IP.To4(); ipv4 != nil {
			var wasiAddr IPv4Address
			copy(wasiAddr[:], ipv4)
			return IPSocketAddress{
				IPV4: &IPv4SocketAddress{
					Port:    uint16(tcpAddr.Port),
					Address: wasiAddr,
				},
			}, nil
		}

		if ipv6 := tcpAddr.IP.To16(); ipv6 != nil {
			var wasiAddr IPv6Address
			for i := 0; i < 8; i++ {
				wasiAddr[i] = binary.BigEndian.Uint16(ipv6[i*2:])
			}
			return IPSocketAddress{
				IPV6: &IPv6SocketAddress{
					Port:    uint16(tcpAddr.Port),
					Address: wasiAddr,
					// FlowInfo and ScopeID require more complex logic to extract
				},
			}, nil
		}
	case *net.UDPAddr:
		if ipv4 := tcpAddr.IP.To4(); ipv4 != nil {
			var wasiAddr IPv4Address
			copy(wasiAddr[:], ipv4)
			return IPSocketAddress{
				IPV4: &IPv4SocketAddress{
					Port:    uint16(tcpAddr.Port),
					Address: wasiAddr,
				},
			}, nil
		}

		if ipv6 := tcpAddr.IP.To16(); ipv6 != nil {
			var wasiAddr IPv6Address
			for i := 0; i < 8; i++ {
				wasiAddr[i] = binary.BigEndian.Uint16(ipv6[i*2:])
			}
			return IPSocketAddress{
				IPV6: &IPv6SocketAddress{
					Port:    uint16(tcpAddr.Port),
					Address: wasiAddr,
					// FlowInfo and ScopeID require more complex logic to extract
				},
			}, nil
		}
	default:
	}
	return IPSocketAddress{}, errors.New("address is not TCPAddr")
}
