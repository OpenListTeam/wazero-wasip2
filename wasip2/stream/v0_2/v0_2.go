package v0_2

import (
	"context"
	"wazero-wasip2/wasip2"
	witgo "wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

type wasiIO struct{}

// New 是 wasiIO 的构造函数。
func New() wasip2.Implementation {
	return &wasiIO{}
}

// Name 返回 WIT 包名。
func (i *wasiIO) Name() string { return "wasi:io/streams" }

// Versions 返回此实现兼容的所有 WIT 版本。
func (i *wasiIO) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

// Instantiate 将所有 streams 函数注册到 wazero。
func (i *wasiIO) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
	// 创建处理器实例，并注入所有依赖项。
	handler := newStreamsImpl(h.StreamManager(), h.ErrorManager(), h.PollManager())
	exporter := witgo.NewExporter(builder)

	// 导出资源析构函数
	exporter.Export("[resource-drop]input-stream", handler.DropInputStream)
	exporter.Export("[resource-drop]output-stream", handler.DropOutputStream)

	// 导出 input-stream 的方法
	exporter.Export("[method]input-stream.read", handler.Read)
	exporter.Export("[method]input-stream.blocking-read", handler.BlockingRead)
	exporter.Export("[method]input-stream.skip", handler.Skip)
	exporter.Export("[method]input-stream.blocking-skip", handler.BlockingSkip)
	exporter.Export("[method]input-stream.subscribe", handler.SubscribeToInputStream)

	// 导出 output-stream 的方法
	exporter.Export("[method]output-stream.check-write", handler.CheckWrite)
	exporter.Export("[method]output-stream.write", handler.Write)
	exporter.Export("[method]output-stream.blocking-write-and-flush", handler.BlockingWriteAndFlush)
	exporter.Export("[method]output-stream.flush", handler.Flush)
	exporter.Export("[method]output-stream.blocking-flush", handler.BlockingFlush)
	exporter.Export("[method]output-stream.subscribe", handler.SubscribeToOutputStream)
	exporter.Export("[method]output-stream.write-zeroes", handler.WriteZeroes)
	exporter.Export("[method]output-stream.blocking-write-zeroes-and-flush", handler.BlockingWriteZeroesAndFlush)
	exporter.Export("[method]output-stream.splice", handler.Splice)
	exporter.Export("[method]output-stream.blocking-splice", handler.BlockingSplice)

	return nil
}
