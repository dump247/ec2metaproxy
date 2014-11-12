package main

type StringSet map[string]bool

func NewStringSet() StringSet {
	return make(StringSet)
}

func (t StringSet) Add(value string) {
	t[value] = true
}

func (t StringSet) Contains(value string) bool {
	if v, f := t[value]; f {
		return v
	}

	return false
}
