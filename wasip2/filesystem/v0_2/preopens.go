package v0_2

import (
	"context"
	"wazero-wasip2/internal/filesystem"
	witgo "wazero-wasip2/wit-go"
)

type preopensImpl struct {
	fsm *filesystem.Manager
}

func newPreopensImpl(fsm *filesystem.Manager) *preopensImpl {
	return &preopensImpl{fsm: fsm}
}

// GetDirectories returns the list of pre-opened directories.
func (i *preopensImpl) GetDirectories(_ context.Context) []witgo.Tuple[Descriptor, string] {
	var results []witgo.Tuple[Descriptor, string]
	// Note: In a real implementation, this would iterate over pre-opened
	// directories configured by the host environment. For this example,
	// we assume the manager contains only pre-opens.
	i.fsm.Range(func(handle uint32, desc *filesystem.Descriptor) bool {
		results = append(results, witgo.Tuple[Descriptor, string]{
			F0: handle,
			F1: desc.Path,
		})
		return true
	})
	return results
}
