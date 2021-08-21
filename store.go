package main

import (
	"sync"
	"time"
)

type storeEntry struct {
	CbkCh     chan FrontendResponse
	CreatedAt time.Time
}

type memStore struct {
	*sync.Mutex
	m map[string]storeEntry
}

func (s *memStore) Put(key string, storeEntry storeEntry) {
	s.Lock()
	defer s.Unlock()

	s.m[key] = storeEntry
}

func (s *memStore) Get(key string) (storeEntry, bool) {
	s.Lock()
	defer s.Unlock()

	ch, ok := s.m[key]
	return ch, ok
}

func (s *memStore) Delete(key string) {
	s.Lock()
	defer s.Unlock()

	delete(s.m, key)
}

var store = &memStore{
	&sync.Mutex{},
	make(map[string]storeEntry),
}
