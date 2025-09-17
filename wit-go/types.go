package witgo

type Unit struct{}

type Option[T any] struct {
	None *Unit `wit:"case(0)"` // Case 0: "none", has no payload
	Some *T    `wit:"case(1)"` // Case 1: "some", has a payload of type T
}

func (o Option[T]) IsOption() {}

// Some creates an option with a value.
func Some[T any](value T) Option[T] {
	return Option[T]{Some: &value}
}

// Some creates an option with a value.
func SomePtr[T any](value T) *Option[T] {
	return &Option[T]{Some: &value}
}

// None creates an option with no value.
func None[T any]() Option[T] {
	return Option[T]{None: &Unit{}}
}

// None creates an option with no value.
func NonePtr[T any]() *Option[T] {
	return &Option[T]{None: &Unit{}}
}

// VariantResult demonstrates implementing a result using the variant structure.
// It is ABI-equivalent to result<T, E>.
type Result[T, E any] struct {
	Ok  *T `wit:"case(0)"` // Case 0: "ok", has a payload of type T
	Err *E `wit:"case(1)"` // Case 1: "error", has a payload of type E
}

func (r Result[T, E]) IsResult() {}

// Ok creates a successful result.
func Ok[T any, E any](value T) Result[T, E] {
	return Result[T, E]{Ok: &value}
}

// Err creates an error result.
func Err[T any, E any](err E) Result[T, E] {
	return Result[T, E]{Err: &err}
}

type Tuple[T0, T1 any] struct {
	F0 T0
	F1 T1
}

type Tuple3[T0, T1, T2 any] struct {
	F0 T0
	F1 T1
	F2 T2
}

func String(s string) *string {
	return &s
}

type UnitResult = uint32

// Ok creates a successful result.
func UintOk() UnitResult {
	return 0
}

// Err creates an error result.
func UintErr() UnitResult {
	return 1
}
