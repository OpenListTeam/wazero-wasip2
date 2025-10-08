package io

import witgo "github.com/foxxorcat/wazero-wasip2/wit-go"

type ErrorManager = witgo.ResourceManager[error]

func NewErrorManager() *ErrorManager {
	return witgo.NewResourceManager[error]()
}
