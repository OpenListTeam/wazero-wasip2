package tests

import (
	"context"
	"io"
	"os"
	"testing"
	"time"
	"wazero-wasip2/internal/streams"
	"wazero-wasip2/wasip2"
	wasi_error "wazero-wasip2/wasip2/error"
	wasi_poll "wazero-wasip2/wasip2/poll"
	wasi_stream "wazero-wasip2/wasip2/stream"
	witgo "wazero-wasip2/wit-go"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestWasiIO(t *testing.T) {
	// 读取编译好的 WASM Guest 模块
	wasm, err := os.ReadFile("guest.wasm")
	require.NoError(t, err)

	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	// 实例化 WASI P1，这是 wasm32-wasi 目标所必需的。
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// 创建我们的 wasip2.Host 实例，并启用所有需要的模块。
	h := wasip2.NewHost(
		wasi_error.Module("0.2.0"),
		wasi_poll.Module("0.2.0"),
		wasi_stream.Module("0.2.0"),
	)
	// 将我们的 Host 实现实例化到 wazero 运行时中。
	err = h.Instantiate(ctx, r)
	require.NoError(t, err)

	// 实例化 Guest 模块。
	mod, err := r.InstantiateWithConfig(ctx, wasm, wazero.NewModuleConfig().WithName("test-guest"))
	require.NoError(t, err)

	// 使用 wit-go 创建一个高层级的 Host 包装器，以方便调用 Guest 的导出函数。
	guest, err := witgo.NewHost(mod)
	require.NoError(t, err)

	t.Run("Read from a blocking pipe", func(t *testing.T) {
		// 1. 创建一个 Go 的 io.Pipe，这是一个典型的阻塞型 I/O 资源。
		pr, pw := io.Pipe()

		// 2. 将 PipeReader 封装成我们的 Stream 对象。
		//    我们将 PipeReader 作为 Reader 和 Closer。
		stream := &streams.Stream{Reader: pr, Closer: pr}

		// 3. 将 Stream 对象添加到 StreamManager 中，获得一个句柄。
		//    这个句柄将作为参数传递给 Guest。
		handle := h.StreamManager().Add(stream)
		defer h.StreamManager().Remove(handle) // 测试结束后清理

		// 4. 在一个单独的 goroutine 中，向 PipeWriter 写入数据。
		//    我们稍微延迟一下写入，以模拟真实的网络或文件 I/O 延迟。
		go func() {
			time.Sleep(10 * time.Millisecond)
			_, err := pw.Write([]byte("Hello from Host!"))
			require.NoError(t, err)
			pw.Close() // 关闭写入端，这会使读取端在读完数据后看到 EOF。
		}()

		var result string
		// 5. 调用 Guest 的 test-read-stream 函数，将流的句柄传给它。
		err = guest.Call(ctx, "test-read-stream", &result, handle)
		require.NoError(t, err)

		// 6. 断言 Guest 返回的结果是否与我们写入的一致。
		require.Equal(t, "Hello from Host!", result)
	})
}
