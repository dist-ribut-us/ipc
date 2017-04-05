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

type callbacks struct {
	Map map[uint32]Callback
	sync.RWMutex
}

func newcallbacks() *callbacks {
	return &callbacks{
		Map: make(map[uint32]Callback),
	}
}

func (t *callbacks) get(key uint32) (Callback, bool) {
	t.RLock()
	k, b := t.Map[key]
	t.RUnlock()
	return k, b
}

func (t *callbacks) set(key uint32, val Callback) {
	t.Lock()
	t.Map[key] = val
	t.Unlock()
}

func (t *callbacks) delete(keys ...uint32) {
	t.Lock()
	for _, key := range keys {
		delete(t.Map, key)
	}
	t.Unlock()
}


