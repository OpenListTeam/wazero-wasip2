package io

import (
	"errors"
	"io"
)

// MultiCloser 用于合并多个io.Closer，统一管理关闭操作
type MultiCloser struct {
	closers []io.Closer
}

// NewMultiCloser 创建一个新的MultiCloser，自动过滤nil值
func NewMultiCloser(closers ...io.Closer) *MultiCloser {
	mc := &MultiCloser{
		closers: make([]io.Closer, 0, len(closers)),
	}

	// 过滤掉nil的closer，避免后续关闭时panic
	for _, c := range closers {
		if c != nil {
			mc.closers = append(mc.closers, c)
		}
	}

	return mc
}

// Close 关闭所有非nil的io.Closer，返回合并后的错误
func (m *MultiCloser) Close() error {
	var errList []error

	// 遍历所有closer并关闭
	for _, c := range m.closers {
		// 再次检查是否为nil（理论上这里不会有nil，因为创建时已过滤）
		if c != nil {
			if err := c.Close(); err != nil {
				errList = append(errList, err)
			}
		}
	}

	// 合并所有错误（Go 1.20+支持errors.Join）
	return errors.Join(errList...)
}
