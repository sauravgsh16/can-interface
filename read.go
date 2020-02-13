package can

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	sizebyte = 1
)

type decoder struct {
	r   io.Reader
	err error
	in  []byte
	out [1024]byte
}

func newDecoder(r io.Reader) *decoder {
	return &decoder{
		r: r,
	}
}

type invalidByte byte

func (i invalidByte) Error() string {
	return fmt.Sprintf("encoding error: invalid byte: %#U", rune(i))
}

func fromHexChar(ch byte) (byte, bool) {
	switch {
	case '0' <= ch && ch <= '9':
		return ch - '0', true
	case 'a' <= ch && ch <= 'f':
		return ch - 'a' + 10, true
	case 'A' <= ch && ch <= 'F':
		return ch - 'A' + 10, true
	default:
		return 0, false
	}
}

func decode(dst, src []byte) (int, int, error) {
	i, j, k := 0, 0, 0
	for i < len(src) && j < len(dst) {
		if src[i] == ' ' {
			i++
			k++
			continue
		}
		a, ok := fromHexChar(src[i])
		if !ok {
			return j, k, invalidByte(src[i])
		}
		b, ok := fromHexChar(src[i+1])
		if !ok {
			return j, k, invalidByte(src[i+1])
		}

		dst[j] = (a << 4) | b
		i += 2
		j++
	}

	return j, k, nil
}

func (d *decoder) Read(p []byte) (n int, err error) {
	var numRead int

	numRead, d.err = io.ReadFull(d.r, d.out[:len(p)*2+1])
	d.in = d.out[:numRead]
	if d.err == io.EOF && len(d.in)%2 != 0 {
		if _, ok := fromHexChar(d.in[len(d.in)-1]); !ok {
			d.err = invalidByte(d.in[len(d.in)-1])
		}
	}

	// After reading data, we decode it
	if numAvail := len(d.in) / 2; len(p) > numAvail {
		p = p[:numAvail]
	}

	numdec, numspace, err := decode(p, d.in)
	d.in = d.in[(numdec*2)+numspace:]

	if len(d.in) > 0 {
		if numdec != len(p) && err != nil {
			return numdec, fmt.Errorf("error: %s, read: %d bytes", err.Error(), numdec)
		}
	}

	return numdec + numspace, nil
}

type reader struct {
	r io.Reader
	d *decoder
}

func newReader(r io.Reader) *reader {
	return &reader{
		r: r,
		d: newDecoder(r),
	}
}

func (r reader) readMessage() (*Message, error) {
	m := newMsg()
	incoming := make([]byte, 7)

	if _, err := io.ReadFull(r.r, incoming[:7]); err != nil {
		return nil, err
	}

	if _, err := r.readArbitrationID(m); err != nil {
		return nil, err
	}

	if err := m.parseArbitrationID(); err != nil {
		return nil, err
	}

	if _, err := r.readSize(m); err != nil {
		return nil, err
	}

	if _, err := r.readBody(m); err != nil {
		return nil, err
	}

	var carriage byte
	if err := binary.Read(r.r, binary.BigEndian, &carriage); err != nil {
		return nil, err
	}

	return m, nil
}

func (r reader) readArbitrationID(m *Message) (int, error) {
	n, err := r.d.Read(m.ArbitrationID)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r reader) readSize(m *Message) (int, error) {
	b := make([]byte, sizebyte)
	n, err := r.d.Read(b)
	if err != nil {
		return 0, err
	}
	m.Size = b[0]
	return n, err
}

func (r reader) readBody(m *Message) (int, error) {
	b := make([]byte, 11)
	n, err := r.d.Read(b)
	if err != nil {
		return 0, err
	}
	m.Data = b[:m.Size]
	return n, nil
}
