package lock

import (
	"sync"
)

type Locker interface {
	Lock(name string)
	Unlock(name string)
	Delete(name string)
}

type lock struct {
	rw    *sync.RWMutex
	locks map[string]*sync.Mutex
}

func NewLock() *lock {
	return &lock{
		rw:    new(sync.RWMutex),
		locks: make(map[string]*sync.Mutex),
	}
}

func (l *lock) Lock(name string) {
	l.rw.Lock()

	lock := l.locks[name]

	if lock == nil {
		// create new object lock

		lock = new(sync.Mutex)

		l.locks[name] = lock
	}

	l.rw.Unlock()

	lock.Lock()
}

func (l *lock) Unlock(name string) {
	l.rw.RLock()

	lock := l.locks[name]

	l.rw.RUnlock()

	if lock != nil {
		lock.Unlock()
	}
}

func (l *lock) Delete(name string) {
	l.rw.Lock()

	delete(l.locks, name)

	l.rw.Unlock()
}
