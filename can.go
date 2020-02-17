package can

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"sync"
)

// Can struct
type Can struct {
	r            *reader
	w            *writer
	conn         *Connection
	currTP       *TP
	In           chan *Message
	Out          chan *Message
	Done         chan bool
	wg           sync.WaitGroup
	ServIncoming chan DataHolder
}

// New returns a pointer to Can
func New(url string, in chan DataHolder, simName string) (*Can, error) {
	conn, err := Dial(url)
	if err != nil {
		return nil, err
	}
	buf := bufio.NewReader(conn.Conn)

	c := &Can{
		conn:         conn,
		r:            newReader(buf),
		w:            newWriter(conn.writer.w),
		In:           make(chan *Message),
		Out:          make(chan *Message),
		Done:         make(chan bool),
		ServIncoming: in,
	}

	if _, err := conn.Conn.Write([]byte(simName)); err != nil {
		return nil, err
	}

	return c, nil
}

// Init can goroutines
func (c *Can) Init() {
	go c.handleIncoming()
	go c.processIncoming()
	go c.handleOutgoing()
	go c.handleClose()
}

func (c *Can) handleClose() {
	select {
	case <-c.Done:
		c.conn.close()
	}
}

func (c *Can) handleIncoming() {
	for {
		if c.conn.isClosed() {
			break
		}
		msg, err := c.r.readMessage()
		if err == io.EOF {
			return
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok {
				log.Printf("closing connection: %s\n", netErr)
				break
			}
			// TODO: better error handling
			log.Printf(err.Error())
			continue
		}

		c.In <- msg
	}
	close(c.In)
}

func (c *Can) processIncoming() {
	handle := make(chan DataHolder)

	var (
		fc   int
		size int
		err  error
	)

	go func() {
	loop:
		for {
			select {

			case msg, ok := <-c.In:
				if !ok {
					break loop
				}

				switch {
				case isPrefix(msg.PGN, "EC"):
					if c.currTP != nil {
						c.wg.Wait()
					}

					if c.currTP, err = newTp(msg); err != nil {
						continue
					}
					size = int(c.currTP.size)
					c.wg.Add(1)

				case isPrefix(msg.PGN, "EB"):
					if c.currTP == nil {
						log.Fatalf("invalid tp pgn '%s' received, before receving tp initial tp info", msg.PGN)
					}

					if fc < c.currTP.frames && size > 0 {
						l := len(msg.Data[1:])

						if size >= l {
							c.currTP.append(msg.Data[1:])
							size -= l
						} else {
							c.currTP.append(msg.Data[1 : size+1])
							size = 0
						}
						fc++
					}

					if fc >= c.currTP.frames {
						// fmt.Printf("%#v\n", c.currTP)

						handle <- c.currTP

						c.currTP = nil
						fc = 0
						size = 0
						c.wg.Done()
					}
				default:
					handle <- msg
				}
			}
		}
		close(handle)
	}()
	go c.handleMessage(handle)
}

func (c *Can) handleMessage(h chan DataHolder) {
	for {
		select {
		case tp, ok := <-h:
			if !ok {
				break
			}

			c.ServIncoming <- tp
		}
	}
}

func (c *Can) handleOutgoing() {
loop:
	for {
		select {
		case m := <-c.Out:
			if c.conn.isClosed() {
				break loop
			}
			go c.write(m)
		}
	}
	close(c.Out)
}

func (c *Can) write(m *Message) error {
	if err := c.w.writeMessage(m); err != nil {
		log.Printf(err.Error())
		return err
	}
	return nil
}

// Close can connection
func (c *Can) Close() {
	select {
	case c.Done <- true:

	default:
		log.Println("Done channel not available")
	}
}

func isPrefix(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}

	i := 0
	s = strings.ToUpper(s)
	for i < len(substr) {
		if s[i] != substr[i] {
			return false
		}
		i++
	}
	return true
}
