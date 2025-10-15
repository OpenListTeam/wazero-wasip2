package io

import witgo "github.com/OpenListTeam/wazero-wasip2/wit-go"

type ErrorManager = witgo.ResourceManager[error]

func NewErrorManager() *ErrorManager {
	return witgo.NewResourceManager[error](nil)
}
