package v0_2

import (
	"context"

	witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"
)

type insecureSeedImpl struct{}

func newInsecureSeedImpl() *insecureSeedImpl {
	return &insecureSeedImpl{}
}

// InsecureSeed 实现了 insecure-seed 函数。
// 它返回一个 128 位的值（作为两个 u64），用于初始化哈希表等。
// 这里我们简单地重用 insecure 的实现来生成种子。
func (i *insecureSeedImpl) InsecureSeed(ctx context.Context) witgo.Tuple[uint64, uint64] {
	// 我们可以调用 crypto/rand 或 math/rand。
	// 规范建议主机实现应提供伪随机值，因此 math/rand 更符合“insecure”的语义。
	insecure := newInsecureImpl()
	return witgo.Tuple[uint64, uint64]{
		F0: insecure.GetInsecureRandomU64(ctx),
		F1: insecure.GetInsecureRandomU64(ctx),
	}
}
