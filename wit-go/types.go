package witgo

type Unit struct{}

type Option[T any] struct {
	None *Unit `wit:"case(0)"` // Case 0: "none", has no payload
	Some *T    `wit:"case(1)"` // Case 1: "some", has a payload of type T
}

// Some creates an option with a value.
func Some[T any](value T) Option[T] {
	return Option[T]{Some: &value}
}

// None creates an option with no value.
func None[T any]() Option[T] {
	return Option[T]{None: &Unit{}}
}

// VariantResult demonstrates implementing a result using the variant structure.
// It is ABI-equivalent to result<T, E>.
type Result[T, E any] struct {
	Ok  *T `wit:"case(0)"` // Case 0: "ok", has a payload of type T
	Err *E `wit:"case(1)"` // Case 1: "error", has a payload of type E
}

// Ok creates a successful result.
func Ok[T any, E any](value T) Result[T, E] {
	return Result[T, E]{Ok: &value}
}

// Err creates an error result.
func Err[T any, E any](err E) Result[T, E] {
	return Result[T, E]{Err: &err}
}
