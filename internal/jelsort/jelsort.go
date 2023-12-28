// Package jelsort provides custom sorting.
package jelsort

import "sort"

type sorter[E any] struct {
	src []E
	lt  func(left, right E) bool
}

func (s sorter[E]) Len() int {
	return len(s.src)
}

func (s sorter[E]) Swap(i, j int) {
	s.src[i], s.src[j] = s.src[j], s.src[i]
}

func (s sorter[E]) Less(i, j int) bool {
	return s.lt(s.src[i], s.src[j])
}

// By takes the items and uses the provided function to sort the list. The
// function should return true if left is less than (comes before) right.
//
// items will not be modified.
func By[E any](items []E, lt func(left E, right E) bool) []E {
	if len(items) == 0 || lt == nil {
		return items
	}

	s := sorter[E]{
		src: make([]E, len(items)),
		lt:  lt,
	}

	copy(s.src, items)
	sort.Sort(s)
	return s.src
}
