package io

import witgo "wazero-wasip2/wit-go"

type ErrorManager = witgo.ResourceManager[error]

func NewErrorManager() *ErrorManager {
	return witgo.NewResourceManager[error]()
}
