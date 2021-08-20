package main

import (
	"sync"
	"time"
)

type Entry struct {
	CbkCh     chan FrontendResponse
	CreatedAt time.Time
}

type MsgStore struct {
	*sync.Mutex
	m map[string]Entry
}

func (s *MsgStore) Put(key string, entry Entry) {
	s.Lock()
	defer s.Unlock()

	s.m[key] = entry
}

func (s *MsgStore) Get(key string) (Entry, bool) {
	s.Lock()
	defer s.Unlock()

	ch, ok := s.m[key]
	return ch, ok
}

func (s *MsgStore) Delete(key string) {
	s.Lock()
	defer s.Unlock()

	delete(s.m, key)
}

var store = &MsgStore{
	&sync.Mutex{},
	make(map[string]Entry),
}
