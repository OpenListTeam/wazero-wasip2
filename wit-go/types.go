package witgo

// Option represents a WIT option<T>.
type Option[T any] struct {
	HasValue bool
	Value    T
}

// Some creates an option with a value.
func Some[T any](value T) Option[T] {
	return Option[T]{HasValue: true, Value: value}
}

// None creates an option with no value.
func None[T any]() Option[T] {
	return Option[T]{HasValue: false}
}

// Result represents a WIT result<T, E>.
type Result[T, E any] struct {
	IsErr bool
	Ok    T
	Err   E
}

// Ok creates a successful result.
func Ok[T any, E any](value T) Result[T, E] {
	return Result[T, E]{IsErr: false, Ok: value}
}

// Err creates an error result.
func Err[T any, E any](err E) Result[T, E] {
	return Result[T, E]{IsErr: true, Err: err}
}
