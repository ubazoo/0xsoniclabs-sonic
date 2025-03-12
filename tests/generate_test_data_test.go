package tests

import (
	"iter"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// generateTestDataBasedOnModificationCombinations generates all possible
// versions of a given type based on the combinations of modifications.
// Each yielded value is derived by constructing an element of T using the
// constructor function, and then applying a series of modifications to it.
//
// Arguments:
//   - constructor: a function that constructs a new instance of T, for each
//     version to be based on an unmodified instance.
//   - modification: a list of modifications to be applied on emitted values.
//
// Returns:
// - an iterator that yields all possible versions of T based on the combinations
func generateTestDataBasedOnModificationCombinations[T any](
	constructor func() T,
	modification [][]func(T) T,
) iter.Seq[T] {
	return func(yield func(data T) bool) {
		for modifications := range cartesianProduct(modification) {
			cur := constructor()
			for _, m := range modifications {
				cur = m(cur)
			}
			if !yield(cur) {
				return
			}
		}
	}
}

// cartesianProduct generates all possible combinations of elements from the
// provided element lists.
func cartesianProduct[T any](elements [][]T) iter.Seq[[]T] {
	cur := make([]T, 0, len(elements))
	return func(yield func(data []T) bool) {
		_cartesianProductRecursion(cur, elements, yield)
	}
}

func TestCartesianProduct_CountInstantiations(t *testing.T) {
	countInstances := func(pieces [][]int) int {
		iter := cartesianProduct(pieces)
		return len(slices.Collect(iter))
	}

	assert.Equal(t, 1, countInstances(nil))
	assert.Equal(t, 1, countInstances([][]int{{1}}))
	assert.Equal(t, 2, countInstances([][]int{{1}, {1, 2}}))
	assert.Equal(t, 4, countInstances([][]int{{1, 2}, {1, 2}}))
	assert.Equal(t, 4, countInstances([][]int{{1}, {1, 2}, {1, 2}}))
	assert.Equal(t, 6, countInstances([][]int{{1}, {1, 2, 3}, {1, 2}}))
	assert.Equal(t, 6, countInstances([][]int{{1}, {1, 2}, {1, 2, 3}}))
}

func TestCartesianProduct_AcceptsFunctionsAsPieces(t *testing.T) {

	type TestType struct {
		A int
		B int
	}
	type Mod = func(TestType) TestType

	setA := func(a int) Mod {
		return func(t TestType) TestType {
			t.A = a
			return t
		}
	}
	setB := func(a int) Mod {
		return func(t TestType) TestType {
			t.B = a
			return t
		}
	}

	instances := make([]TestType, 0)
	for v := range generateTestDataBasedOnModificationCombinations(
		func() TestType { return TestType{} },
		[][]Mod{
			{setA(1), setA(2)},
			{setB(1), setB(2)},
		}) {
		instances = append(instances, v)
	}

	assert.Len(t, instances, 4)
	assert.Contains(t, instances, TestType{1, 1})
	assert.Contains(t, instances, TestType{1, 2})
	assert.Contains(t, instances, TestType{2, 1})
	assert.Contains(t, instances, TestType{2, 2})
}

func _cartesianProductRecursion[T any](current []T, elements [][]T, yield func([]T) bool) bool {
	if len(elements) == 0 {
		return yield(current)
	}

	rest := elements[1:]
	for _, element := range elements[0] {
		if !_cartesianProductRecursion(append(current, element), rest, yield) {
			return false
		}
	}
	return true
}
