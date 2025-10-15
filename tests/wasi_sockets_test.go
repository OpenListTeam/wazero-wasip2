package tests

import (
	"context"
	"net"
	"os"
	"sync"
	"testing"

	"github.com/OpenListTeam/wazero-wasip2/wasip2"
	wasi_clocks "github.com/OpenListTeam/wazero-wasip2/wasip2/clocks"
	wasi_io "github.com/OpenListTeam/wazero-wasip2/wasip2/io"
	wasi_sockets "github.com/OpenListTeam/wazero-wasip2/wasip2/sockets"
	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// setupSocketsTest a helper function to initialize the wazero runtime and our wasip2 host.
func setupSocketsTest(t *testing.T) (context.Context, *wasip2.Host, *witgo.Host) {
	wasm, err := os.ReadFile("guest.wasm")
	require.NoError(t, err)

	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	t.Cleanup(func() { r.Close(ctx) })

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// Enable all necessary WASI modules for sockets and I/O
	h := wasip2.NewHost(
		wasi_clocks.Module("0.2.0"),
		wasi_io.Module("0.2.0"),
		wasi_sockets.Module("0.2.0"),
	)
	err = h.Instantiate(ctx, r)
	require.NoError(t, err)

	mod, err := r.InstantiateWithConfig(ctx, wasm, wazero.NewModuleConfig().WithName("sockets-guest"))
	require.NoError(t, err)

	guest, err := witgo.NewHost(mod)
	require.NoError(t, err)

	return ctx, h, guest
}

func TestWasiTCPSockets(t *testing.T) {
	ctx, _, guest := setupSocketsTest(t)

	// 1. Set up a TCP listener on the Go host.
	listenAddr := "127.0.0.1:0" // Use port 0 to get a random available port
	listener, err := net.Listen("tcp", listenAddr)
	require.NoError(t, err)
	defer listener.Close()

	// Get the actual address the listener is bound to.
	addr := listener.Addr().(*net.TCPAddr)

	var wg sync.WaitGroup
	wg.Add(1)

	hostMsg := "Hello from host TCP!"
	guestMsg := "Hello from guest TCP!"

	// 2. Start a goroutine to act as the TCP server.
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		require.NoError(t, err)
		defer conn.Close()

		// 2a. Read the message from the guest.
		buffer := make([]byte, len(guestMsg))
		_, err = conn.Read(buffer)
		require.NoError(t, err)
		require.Equal(t, guestMsg, string(buffer))

		// 2b. Send a message back to the guest.
		_, err = conn.Write([]byte(hostMsg))
		require.NoError(t, err)
	}()

	// 3. Call the guest function to connect to the host listener and perform the exchange.
	var result string
	err = guest.Call(ctx, "test-tcp-sockets", &result, uint16(addr.Port), guestMsg)
	require.NoError(t, err)

	// 4. Verify the guest received the host's message correctly.
	require.Equal(t, hostMsg, result)

	// 5. Wait for the server goroutine to finish.
	wg.Wait()
}

func TestWasiUDPSockets(t *testing.T) {
	ctx, _, guest := setupSocketsTest(t)

	// 1. Set up a UDP socket on the Go host.
	listenAddr := "127.0.0.1:0"
	conn, err := net.ListenPacket("udp", listenAddr)
	require.NoError(t, err)
	defer conn.Close()

	addr := conn.LocalAddr().(*net.UDPAddr)

	var wg sync.WaitGroup
	wg.Add(1)

	hostMsg := "Hello from host UDP!"
	guestMsg := "Hello from guest UDP!"

	// 2. Start a goroutine to act as the UDP server.
	go func() {
		defer wg.Done()
		buffer := make([]byte, 1024)

		// 2a. Read a datagram from the guest.
		n, remoteAddr, err := conn.ReadFrom(buffer)
		require.NoError(t, err)
		require.Equal(t, guestMsg, string(buffer[:n]))

		// 2b. Send a datagram back to the guest's address.
		_, err = conn.WriteTo([]byte(hostMsg), remoteAddr)
		require.NoError(t, err)
	}()

	// 3. Call the guest function to send and receive a datagram.
	var result string
	err = guest.Call(ctx, "test-udp-sockets", &result, uint16(addr.Port), guestMsg)
	require.NoError(t, err)

	// 4. Verify the guest received the host's message.
	require.Equal(t, hostMsg, result)

	// 5. Wait for the server goroutine to finish.
	wg.Wait()
}
