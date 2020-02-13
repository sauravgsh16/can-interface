package can

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
)

const (
	broadcastThreshold int = 239
)

var (
	errorEmptyArbitrationID = errors.New("attempt to parse empty arbitration id")
)

// DataHolder interface
type DataHolder interface {
	GetSrc() byte
	GetDst() byte
	GetData() []byte
}

// Message struct
type Message struct {
	ArbitrationID []byte
	Priority      byte
	PGN           string
	Src           byte
	Dst           byte
	Size          byte
	Data          []byte
}

// "Xtd 02 0CCBF782 08 13 00 86 00 B8 0B 00 00\n"

// New Can message
func newMsg() *Message {
	return &Message{
		ArbitrationID: make([]byte, 4),
		Data:          make([]byte, 8),
	}
}

func (m *Message) group() ([]byte, error) {
	b := make([]byte, 0)
	buf := bytes.NewBuffer(b)

	if _, err := buf.Write(m.ArbitrationID); err != nil {
		return nil, err
	}

	if _, err := buf.Write(m.Data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (m *Message) parseArbitrationID() error {
	if len(m.ArbitrationID) == 0 {
		return errorEmptyArbitrationID
	}

	m.Priority = m.ArbitrationID[0]
	m.Src = m.ArbitrationID[3]
	m.Dst = m.ArbitrationID[2]

	var pgn []byte
	if int(m.ArbitrationID[2]) > broadcastThreshold {
		pgn = []byte{m.ArbitrationID[1], 0x00}
	} else {
		pgn = m.ArbitrationID[1:3]
	}

	m.PGN = strings.ToUpper(hex.EncodeToString(pgn))
	return nil
}

// GetSrc returns the Src id of an individual can message
func (m *Message) GetSrc() byte {
	return m.Src
}

// GetDst returns the Dst id of an individual can message
func (m *Message) GetDst() byte {
	return m.Dst
}

// GetData returns the Data contained in an individual can message
func (m *Message) GetData() []byte {
	return m.Data
}

// TP Can Message
type TP struct {
	Pgn    string
	Src    byte
	Dst    byte
	size   int16
	frames int
	Data   []byte
	mux    sync.Mutex
}

func newTp(m *Message) (*TP, error) {
	var s int16
	s = int16(m.Data[2])<<8 | int16(m.Data[1])
	return &TP{
		Pgn:    hex.EncodeToString([]byte{m.Data[6], m.Data[5]}),
		Src:    m.Src,
		Dst:    m.Dst,
		size:   s,
		frames: int(m.Data[3]),
		Data:   make([]byte, 0, int(m.Data[1])),
	}, nil
}

func (t *TP) currSize() int {
	return len(t.Data)
}

func (t *TP) append(d []byte) {
	t.mux.Lock()
	defer t.mux.Unlock()

	t.Data = append(t.Data, d...)
}

func (t *TP) isValid(pgn string) bool {
	return t.Pgn == pgn
}

// GetSrc returns the Src id of the tp message
func (t *TP) GetSrc() byte {
	return t.Src
}

// GetDst returns the Dst id of the tp message
func (t *TP) GetDst() byte {
	return t.Dst
}

// GetData returns the Data contained in the tp message
func (t *TP) GetData() []byte {
	return t.Data
}
