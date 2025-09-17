package v0_2

import (
	"context"
	"crypto/rand"
	"encoding/binary"
)

type randomImpl struct{}

func newRandomImpl() *randomImpl {
	return &randomImpl{}
}

// GetRandomBytes 实现了 get-random-bytes 函数。
func (i *randomImpl) GetRandomBytes(_ context.Context, length uint64) []byte {
	buf := make([]byte, length)
	_, err := rand.Read(buf)
	if err != nil {
		// 根据 WASI Random 规范，此函数不应失败。
		// 在真实世界中，如果 crypto/rand.Read 失败，表明系统存在严重问题。
		// 此时 panic 是一个合理的选择，因为它表示一个不可恢复的错误。
		panic(err)
	}
	return buf
}

// GetRandomU64 实现了 get-random-u64 函数。
func (i *randomImpl) GetRandomU64(_ context.Context) uint64 {
	var buf [8]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint64(buf[:])
}
