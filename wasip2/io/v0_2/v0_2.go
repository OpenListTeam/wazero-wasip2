package v0_2

import (
	"context"
	"wazero-wasip2/wasip2"
	witgo "wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

// --- wasi:io/error@0.2.7 implementation ---

type wasiError struct{}

func NewError() wasip2.Implementation {
	return &wasiError{}
}

func (i *wasiError) Name() string { return "wasi:io/error" }
func (i *wasiError) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiError) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
	handler := newErrorImpl(h.ErrorManager())
	exporter := witgo.NewExporter(builder)
	exporter.Export("[resource-drop]error", handler.DropError)
	exporter.Export("[method]error.to-debug-string", handler.ToDebugString)
	return nil
}

// --- wasi:io/poll@0.2.7 implementation ---

type wasiPoll struct{}

func NewPoll() wasip2.Implementation {
	return &wasiPoll{}
}

func (i *wasiPoll) Name() string { return "wasi:io/poll" }
func (i *wasiPoll) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiPoll) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
	handler := newPollImpl(h.PollManager())
	exporter := witgo.NewExporter(builder)
	exporter.Export("[resource-drop]pollable", handler.DropPollable)
	exporter.Export("[method]pollable.ready", handler.Ready)
	exporter.Export("[method]pollable.block", handler.Block)
	exporter.Export("poll", handler.Poll)
	return nil
}

// --- wasi:io/streams@0.2.7 implementation ---

type wasiStreams struct{}

func NewStreams() wasip2.Implementation {
	return &wasiStreams{}
}

func (i *wasiStreams) Name() string { return "wasi:io/streams" }
func (i *wasiStreams) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiStreams) Instantiate(_ context.Context, h *wasip2.Host, builder wazero.HostModuleBuilder) error {
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
