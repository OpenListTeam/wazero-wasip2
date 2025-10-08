package v0_2

import (
	"context"

	"github.com/foxxorcat/wazero-wasip2/wasip2"
	witgo "github.com/foxxorcat/wazero-wasip2/wit-go"

	"github.com/tetratelabs/wazero"
)

// --- wasi:filesystem/types implementation ---
type wasiTypes struct{}

func NewTypes() wasip2.Implementation {
	return &wasiTypes{}
}

func (i *wasiTypes) Name() string { return "wasi:filesystem/types" }
func (i *wasiTypes) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiTypes) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newTypesImpl(h)
	exporter := witgo.NewExporter(b)

	// --- resource: descriptor ---
	exporter.Export("[resource-drop]descriptor", handler.DropDescriptor)
	exporter.Export("[method]descriptor.read-via-stream", handler.ReadViaStream)
	exporter.Export("[method]descriptor.write-via-stream", handler.WriteViaStream)
	exporter.Export("[method]descriptor.append-via-stream", handler.AppendViaStream)
	exporter.Export("[method]descriptor.advise", handler.Advise)
	exporter.Export("[method]descriptor.sync-data", handler.SyncData)
	exporter.Export("[method]descriptor.get-flags", handler.GetFlags)
	exporter.Export("[method]descriptor.get-type", handler.GetType)
	exporter.Export("[method]descriptor.set-size", handler.SetSize)
	exporter.Export("[method]descriptor.set-times", handler.SetTimes)
	exporter.Export("[method]descriptor.read", handler.Read)
	exporter.Export("[method]descriptor.write", handler.Write)
	exporter.Export("[method]descriptor.read-directory", handler.ReadDirectory)
	exporter.Export("[method]descriptor.sync", handler.Sync)
	exporter.Export("[method]descriptor.create-directory-at", handler.CreateDirectoryAt)
	exporter.Export("[method]descriptor.stat", handler.Stat)
	exporter.Export("[method]descriptor.stat-at", handler.StatAt)
	exporter.Export("[method]descriptor.set-times-at", handler.SetTimesAt)
	exporter.Export("[method]descriptor.link-at", handler.LinkAt)
	exporter.Export("[method]descriptor.open-at", handler.OpenAt)
	exporter.Export("[method]descriptor.readlink-at", handler.ReadlinkAt)
	exporter.Export("[method]descriptor.remove-directory-at", handler.RemoveDirectoryAt)
	exporter.Export("[method]descriptor.rename-at", handler.RenameAt)
	exporter.Export("[method]descriptor.symlink-at", handler.SymlinkAt)
	exporter.Export("[method]descriptor.unlink-file-at", handler.UnlinkFileAt)
	exporter.Export("[method]descriptor.is-same-object", handler.IsSameObject)
	exporter.Export("[method]descriptor.metadata-hash", handler.MetadataHash)
	exporter.Export("[method]descriptor.metadata-hash-at", handler.MetadataHashAt)

	// --- resource: directory-entry-stream ---
	exporter.Export("[resource-drop]directory-entry-stream", handler.DropDirectoryEntryStream)
	exporter.Export("[method]directory-entry-stream.read-directory-entry", handler.ReadDirectoryEntry)

	// --- standalone functions ---
	exporter.Export("filesystem-error-code", handler.FilesystemErrorCode)

	return nil
}

// --- wasi:filesystem/preopens implementation ---
type wasiPreopens struct{}

func NewPreopens() wasip2.Implementation {
	return &wasiPreopens{}
}

func (i *wasiPreopens) Name() string { return "wasi:filesystem/preopens" }
func (i *wasiPreopens) Versions() []string {
	return []string{"0.2.0", "0.2.1", "0.2.2", "0.2.3", "0.2.4", "0.2.5", "0.2.6", "0.2.7"}
}

func (i *wasiPreopens) Instantiate(_ context.Context, h *wasip2.Host, b wazero.HostModuleBuilder) error {
	handler := newPreopensImpl(h.FilesystemManager())
	exporter := witgo.NewExporter(b)
	exporter.Export("get-directories", handler.GetDirectories)
	return nil
}
