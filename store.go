package main

import "sync"

type Entry struct {
	CbkCh chan FrontendResponse
}

type MsgStore struct {
	*sync.Mutex
	m map[string]Entry
}

func (s *MsgStore) Put(messageUUID string, entry Entry) {
	s.Lock()
	defer s.Unlock()

	s.m[messageUUID] = entry
}

func (s *MsgStore) Get(messageUUID string) (Entry, bool) {
	s.Lock()
	defer s.Unlock()

	ch, ok := s.m[messageUUID]
	return ch, ok
}

func (s *MsgStore) Delete(messageUUID string) {
	s.Lock()
	defer s.Unlock()

	delete(s.m, messageUUID)
}

var store = &MsgStore{
	&sync.Mutex{},
	make(map[string]Entry),
}
