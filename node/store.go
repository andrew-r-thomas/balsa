package main

import "fmt"

// TODO: figure out if we want this to be persistent too
type Store struct {
	m map[string]int
}

func NewStore() Store {
	return Store{m: map[string]int{}}
}

type Write struct {
	key string
	val int
}

func (s *Store) ApplyWrites(writes []Write) {
	for _, w := range writes {
		if w.val == -1 {
			delete(s.m, w.key)
		} else {
			s.m[w.key] = w.val
		}
	}
}

func (s *Store) Get(key string) (int, error) {
	if v, ok := s.m[key]; ok {
		return v, nil
	} else {
		return v, fmt.Errorf("key not found")
	}
}
