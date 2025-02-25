package result

// T is a result of an operation that can return a value or an error.
type T[V any] struct {
	value V
	err   error
}

// New creates a new result with a value. This is the type of result that is
// returned when the operation is successful.
func New[V any](value V) T[V] {
	return T[V]{value: value}
}

// Error creates a new result with an error. This is the type of result that is
// returned when the operation is unsuccessful. Note, passing nil will as a
// an error will result in a non-error result containing the zero value.
func Error[V any](err error) T[V] {
	return T[V]{err: err}
}

// Unwrap returns the value and error of the result. If the operation was a
// success, the error will be nil. If the operation was a failure, the value
// will be the zero value of the type and the error will describe the failure.
func (r T[V]) Unwrap() (V, error) {
	return r.value, r.err
}

// IsError returns true if the result is an error.
func (r T[V]) IsError() bool {
	return r.err != nil
}
