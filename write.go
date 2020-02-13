package can

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/go-multierror"
)

const (
	hexchars = "0123456789abcdef"
)

func encode(src byte) []byte {
	dst := make([]byte, 2)
	dst[0] = hexchars[src>>4]
	dst[1] = hexchars[src&0x0F]
	return dst
}

type encoder struct {
	w   io.Writer
	err error
	f   chan bool
	mu  *sync.RWMutex
}

func (e *encoder) Write(p []byte) (n int, err error) {
	if len(p) <= 0 {
		return
	}

	var c int
	var common string

	for c < 3 {
		switch {
		case c < 1:
			common += "xtd "
			c++
		case c < 2:
			common += "1 "
			c++
		case c < 3:
			for _, b := range p[:4] {
				common += string(encode(b))
			}
			common += " "
			c++
			p = p[4:]
		}

	}

	var chunk int = 8
	var s string = common

	switch {
	case len(p) <= chunk:
		s += fmt.Sprintf("%01d ", chunk)
		w, err := e.encodeAndWrite(p, chunk, s)
		if err != nil {
			e.err = err
		}
		n += w / 2

	case len(p) > chunk:
		var counter uint8 = 1
		chunk = chunk - 1

		for len(p) > 0 && e.err == nil {
			if len(p)%chunk != 0 && len(p) < chunk {
				chunk = len(p) % chunk
			}

			s += fmt.Sprintf("%02d %02d ", chunk+1, counter)

			w, err := e.encodeAndWrite(p, chunk, s)
			if err != nil {
				e.err = err
			}
			n += w / 2

			p = p[chunk:]
			s = common
			counter++
		}
	}

	e.f <- true

	return n, e.err
}

func (e *encoder) encodeAndWrite(p []byte, chunk int, s string) (int, error) {
	var i int
	for i = 0; i < chunk-1; i++ {
		s += string(encode(p[i]))
		s += " "
	}
	s += string(encode(p[i]))
	s += "\n"

	fmt.Printf("%s", s)

	e.mu.RLock()
	w, err := e.w.Write([]byte(s))
	e.mu.RUnlock()

	return w, err
}

func newEncoder(w io.Writer, f chan bool, mu *sync.RWMutex) *encoder {
	return &encoder{
		w:  w,
		f:  f,
		mu: mu,
	}
}

type writer struct {
	w       io.Writer
	e       *encoder
	flushCh chan bool
	done    chan bool
	errCh   chan error
	mu      *sync.RWMutex
}

func newWriter(w io.Writer) *writer {
	var mu sync.RWMutex
	f := make(chan bool)
	e := newEncoder(w, f, &mu)

	wtr := &writer{
		w:       w,
		e:       e,
		flushCh: f,
		errCh:   make(chan error),
		done:    make(chan bool),
		mu:      &mu,
	}
	return wtr
}

func (w *writer) writeMessage(m *Message) error {
	var err error

	b, err := m.group()
	if err != nil {
		return err
	}

	go w.flush()

	if _, wErr := w.e.Write(b); wErr != nil {
		err = multierror.Append(err, wErr)
	}

	select {
	case fErr := <-w.errCh:
		if fErr != nil {
			err = multierror.Append(err, fErr)
		}
	}

	return err
}

func (w *writer) flush() {
	select {
	case <-w.flushCh:
		if buf, ok := w.w.(*bufio.Writer); ok {
			w.mu.Lock()
			if err := buf.Flush(); err != nil {
				w.errCh <- err
			}
			w.mu.Unlock()
		}
		w.errCh <- nil
	}
}
