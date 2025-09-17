package errors

import (
	witgo "wazero-wasip2/wit-go"
)

type ResourceManager = witgo.ResourceManager[error]

func NewManager() *ResourceManager {
	return witgo.NewResourceManager[error]()
}
