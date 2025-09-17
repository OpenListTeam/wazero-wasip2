package v0_2

import (
	"context"
	"math/rand"
	"time"
)

// insecureImpl 实现了 wasi:random/insecure 接口。
// 它为每个实例包含一个独立的、正确初始化的随机数生成器。
type insecureImpl struct {
	r *rand.Rand
}

// newInsecureImpl 创建一个新的 insecureImpl 实例。
// 它使用 rand.NewSource 来创建一个新的、非共享的随机数源，
// 这是当前推荐的最佳实践。
func newInsecureImpl() *insecureImpl {
	return &insecureImpl{
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetInsecureRandomBytes 实现了 get-insecure-random-bytes 函数。
func (i *insecureImpl) GetInsecureRandomBytes(_ context.Context, length uint64) []byte {
	buf := make([]byte, length)
	// 使用实例独有的 rand.Rand 对象来填充缓冲区
	_, err := i.r.Read(buf)
	if err != nil {
		// rand.Rand.Read 理论上不应该返回错误
		panic(err)
	}
	return buf
}

// GetInsecureRandomU64 实现了 get-insecure-random-u64 函数。
func (i *insecureImpl) GetInsecureRandomU64(_ context.Context) uint64 {
	// 使用实例独有的 rand.Rand 对象来生成 u64
	return i.r.Uint64()
}
