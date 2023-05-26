package gstorage

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/alrusov/jsonw"
)

//----------------------------------------------------------------------------------------------------------------------------//

func Test1(t *testing.T) {
	type testT struct {
		x int
		y int
	}

	n := 100

	s := New[*testT](n)

	//---------//

	for i := 0; i < n; i++ {
		s.Add(&testT{x: i})
	}

	if s.Len() != n {
		t.Fatalf("Fill: Len= %d, expected: %d", s.Len(), n)
	}

	//---------//

	n1, err := s.Enumerate(
		func(idx int, elem *testT) (action EnumeratorAction, err error) {
			elem.y = elem.x
			action = EnumeratorActionContinue
			if idx%10 == 0 {
				action = EnumeratorActionDelete
			}
			return
		},
		true,
	)

	if err != nil {
		t.Fatalf("Enumerate1: %s", err)
	}

	if n1 != n {
		t.Fatalf("Enumerate1: %d iteration, expected: %d", n1, n)
	}

	n1 = n - n/10
	if n%10 != 0 {
		n1--
	}
	if s.Len() != n1 {
		t.Fatalf("Enumerate1: Len=%d, expected: %d", s.Len(), n1)
	}

	//---------//

	n2, err := s.Enumerate(
		func(idx int, elem *testT) (action EnumeratorAction, err error) {
			action = EnumeratorActionContinue
			if elem.x != elem.y {
				action = EnumeratorActionFinish
				err = fmt.Errorf("%d - %d != %d", idx, elem.x, elem.y)
			}
			return
		},
		true,
	)

	if err != nil {
		t.Fatalf("Enumerate2: %s", err)
	}

	if n2 != n1 {
		t.Fatalf("Enumerate2: %d iteration, expected: %d", n2, n1)
	}

	//---------//

	for idx := 0; idx < s.Len(); idx++ {
		elem, exists := s.Get(idx)
		if !exists {
			t.Fatalf("Get(%d): not exists", idx)
		}

		if elem.x != elem.y {
			t.Fatalf("Get(%d) - %d != %d", idx, elem.x, elem.y)
		}
	}

	//---------//

	all := s.GetAll()
	if len(all) != n2 {
		t.Fatalf("GetAll: Len=%d, expected: %d", s.Len(), n2)
	}

	//---------//

	elem, exists := s.Pop()
	if !exists {
		t.Fatalf("Pop(): no fdata found")
	}

	if elem.x != elem.y {
		t.Fatalf("Pop() - %d != %d", elem.x, elem.y)
	}

	n2--

	//---------//

	elem2 := &testT{x: -1, y: -2}

	err = s.Replace(n2+1, elem2)
	if err == nil {
		t.Fatalf("Replace() no error returned, expected error")
	}

	err = s.Replace(n2-1, elem2)
	if err != nil {
		t.Fatalf("Replace(): %s", err)
	}

	elem, exists = s.Get(n2 - 1)

	if !exists {
		t.Fatalf("Get(%d): not exists", n2-1)
	}

	if !reflect.DeepEqual(elem2, elem) {
		t.Fatalf("Get(%d): got: %#v, expected: %#v", n2-1, elem2, elem)
	}
}

//----------------------------------------------------------------------------------------------------------------------------//

func TestJSONlist(t *testing.T) {
	type testT struct {
		X int
		S string
	}

	n := 100

	s := New[*testT](n)

	for i := 0; i < n; i++ {
		s.Add(&testT{X: i, S: fmt.Sprintf("x=%d", i)})
	}

	exp := make([]byte, 0, s.Len()*100)
	_, err := s.Enumerate(
		func(idx int, elem *testT) (action EnumeratorAction, err error) {
			j, err := jsonw.Marshal(elem)
			if err != nil {
				t.Fatal(err)
			}

			exp = append(exp, j...)
			exp = append(exp, '\n')

			action = EnumeratorActionContinue
			return
		},
		true,
	)

	exp = exp[:len(exp)-1] // remove last \n

	if err != nil {
		t.Fatal(err)
	}

	j, err := s.JSONlist()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(j, exp) {
		t.Errorf("\ngot\n%.1000s...\nexpected\n%.1000s...", j, exp)
	}
}

//----------------------------------------------------------------------------------------------------------------------------//

func TestFromJSONlist(t *testing.T) {
	type testT struct {
		X int
		S string
	}

	n := 100

	s := New[*testT](n)

	for i := 0; i < n; i++ {
		s.Add(&testT{X: i, S: fmt.Sprintf("x=%d", i)})
	}

	j, err := s.JSONlist()
	if err != nil {
		t.Fatal(err)
	}

	s2 := New[*testT](0)

	err = s2.FromJSONlist(j)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(s2, s) {
		js, _ := jsonw.Marshal(s.list)
		js2, _ := jsonw.Marshal(s2.list)
		t.Errorf("\ngot\n%.1000s...\nexpected\n%.1000s...", js2, js)
	}
}

//----------------------------------------------------------------------------------------------------------------------------//

func TestMulti(t *testing.T) {
	type testT struct {
		X int
	}

	s := New[*testT](0)

	n := 1000000

	wb := new(sync.WaitGroup)
	wb.Add(n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			s.Add(&testT{X: i})
			wb.Done()
		}()
	}

	wb.Wait()

	if s.Len() != n {
		t.Fatalf("length is %d, expected %d", s.Len(), n)
	}

	m := make(map[int]bool, n)
	for i := 0; i < n; i++ {
		m[i] = true
	}

	for i := 0; i < n; i++ {
		v, exists := s.Get(i)
		if !exists {
			t.Errorf("value with index %d not found", i)
			continue
		}

		if v == nil {
			t.Errorf("nil value")
			continue
		}

		b, exists := m[v.X]
		if !exists {
			t.Errorf("illegal value %d", v.X)
			continue
		}

		if !b {
			t.Errorf("duplicated value %d", v.X)
			continue
		}

		m[v.X] = false
	}
}

//----------------------------------------------------------------------------------------------------------------------------//
