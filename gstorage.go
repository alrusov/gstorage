package gstorage

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/alrusov/jsonw"
	"github.com/alrusov/workers"
)

//----------------------------------------------------------------------------------------------------------------------------//

type (
	S[T any] struct {
		mutex *sync.RWMutex
		list  []T
	}

	Enumerator[T any] func(idx int, elem T) (action EnumeratorAction, err error)

	EnumeratorAction int
)

const (
	EnumeratorActionContinue EnumeratorAction = iota
	EnumeratorActionDelete
	EnumeratorActionFinish
)

//----------------------------------------------------------------------------------------------------------------------------//

func New[T any](capacity int) *S[T] {
	return &S[T]{
		mutex: new(sync.RWMutex),
		list:  make([]T, 0, capacity),
	}
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Lock() {
	s.mutex.Lock()
}

func (s *S[T]) Unlock() {
	s.mutex.Unlock()
}

func (s *S[T]) RLock() {
	s.mutex.RLock()
}

func (s *S[T]) RUnlock() {
	s.mutex.RUnlock()
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Reset(capacity int) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	s.list = make([]T, 0, capacity)
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Len() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return len(s.list)
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Add(o T) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.list = append(s.list, o)
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Set(o []T) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.list = o
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Append(o *S[T]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.list = append(s.list, o.list...)
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Get(idx int) (elem T, exists bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	exists = idx >= 0 && idx < len(s.list)
	if !exists {
		return
	}

	elem = s.list[idx]
	return
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) GetAll() []T {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.list
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Pop() (elem T, exists bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	ln := len(s.list)

	exists = ln > 0
	if !exists {
		return
	}

	elem = s.list[0]

	if ln == 1 {
		s.list = make([]T, cap(s.list))
		return
	}

	s.list = s.list[1:]

	return
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Replace(idx int, elem T) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if idx < 0 || idx >= len(s.list) {
		err = fmt.Errorf("illegal index %d, expected between 0 and %d", idx, len(s.list))
		return
	}

	s.list[idx] = elem
	return
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) Enumerate(f Enumerator[T], forUpdate bool) (n int, err error) {
	if forUpdate {
		s.mutex.Lock()
		defer s.mutex.Unlock()
	} else {
		s.mutex.RLock()
		defer s.mutex.RUnlock()
	}

	if len(s.list) == 0 {
		return
	}

	toDelete := map[int]bool{}
	firstDelIdx := -1

	defer func() {
		if firstDelIdx < 0 {
			return
		}

		dstIdx := firstDelIdx

		for srcIdx := firstDelIdx + 1; srcIdx < len(s.list); srcIdx++ {
			if _, del := toDelete[srcIdx]; del {
				continue
			}

			s.list[dstIdx] = s.list[srcIdx]
			dstIdx++
		}

		s.list = s.list[:dstIdx]
	}()

	var action EnumeratorAction

	var elem T
	for n, elem = range s.list {
		action, err = f(n, elem)
		if err != nil {
			return
		}

		switch action {
		case EnumeratorActionContinue:
			continue

		case EnumeratorActionDelete:
			if firstDelIdx < 0 {
				firstDelIdx = n
			}
			toDelete[n] = true

		case EnumeratorActionFinish:
			return
		}
	}

	n++

	return
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) JSON() (j []byte, err error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return jsonw.Marshal(s.list)
}

//----------------------------------------------------------------------------------------------------------------------------//

func (s *S[T]) JSONlist() (j []byte, err error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(s.list) == 0 {
		return
	}

	m := &m[T]{
		list: s.list,
		j:    make([][]byte, len(s.list)),
	}

	w, err := workers.New(m, workers.Flags(workers.FlagDontUseGetElement))
	if err != nil {
		return
	}

	err = w.Do()
	if err != nil {
		return
	}

	return append(bytes.Join(m.j, []byte{'\n'}), '\n'), nil
}

type m[T any] struct {
	list []T
	j    [][]byte
}

func (h *m[T]) ElementsCount() int {
	return len(h.list)
}

func (h *m[T]) GetElement(idx int) interface{} {
	return nil
}

func (h *m[T]) ProcInitFunc(workerID int) {
}

func (h *m[T]) ProcFinishFunc(workerID int) {
}

func (h *m[T]) ProcFunc(idx int, _ interface{}) (err error) {
	j, err := jsonw.Marshal(h.list[idx])
	if err != nil {
		return
	}

	h.j[idx] = j
	return
}

//----------------------------------------------------------------------------------------------------------------------------//
