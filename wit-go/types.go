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

// IsSome returns `true` if the option is a `Some` value.
func (o Option[T]) IsSome() bool {
	return o.Some != nil
}

// IsNone returns `true` if the option is a `None` value.
func (o Option[T]) IsNone() bool {
	return o.Some == nil
}

// Value returns the contained `Some` value and `true`,
// or the zero value of T and `false` if the option is `None`.
// 这是 Go 语言中最符合习惯的取值方式，类似于 map 的 "comma ok" 用法。
func (o Option[T]) Value() (T, bool) {
	if o.IsSome() {
		return *o.Some, true
	}
	var zero T
	return zero, false
}

// Expect returns the contained `Some` value.
// Panics with the given message if the value is `None`.
func (o Option[T]) Expect(msg string) T {
	if o.IsSome() {
		return *o.Some
	}
	panic(msg)
}

// Unwrap returns the contained `Some` value.
// Panics if the value is a `None` with a default message.
// 推荐在能明确知道值存在的情况下使用，否则请使用更安全的方法。
func (o Option[T]) Unwrap() T {
	return o.Expect("called `Unwrap()` on a `None` value")
}

// UnwrapOr returns the contained `Some` value or a provided default.
func (o Option[T]) UnwrapOr(defaultValue T) T {
	if o.IsSome() {
		return *o.Some
	}
	return defaultValue
}

// UnwrapOrElse returns the contained `Some` value or computes it from a closure.
// 当默认值的计算成本较高时，这个方法很有用。
func (o Option[T]) UnwrapOrElse(f func() T) T {
	if o.IsSome() {
		return *o.Some
	}
	return f()
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
