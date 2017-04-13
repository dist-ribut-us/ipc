package ipc

import (
  "sync"
)

type packets struct {
	Map map[uint32]*Package
	sync.RWMutex
}

func newpackets() *packets {
	return &packets{
		Map: make(map[uint32]*Package),
	}
}

func (t *packets) get(key uint32) (*Package, bool) {
	t.RLock()
	k, b := t.Map[key]
	t.RUnlock()
	return k, b
}

func (t *packets) set(key uint32, val *Package) {
	t.Lock()
	t.Map[key] = val
	t.Unlock()
}

func (t *packets) delete(keys ...uint32) {
	t.Lock()
	for _, key := range keys {
		delete(t.Map, key)
	}
	t.Unlock()
}


