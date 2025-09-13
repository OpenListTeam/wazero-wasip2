package witgo

// align aligns ptr to the given alignment.
func align(ptr, alignment uint32) uint32 {
	return (ptr + alignment - 1) &^ (alignment - 1)
}
