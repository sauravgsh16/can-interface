package can

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync"
	"time"
)

const (
	tcpTimeout = 30 * time.Second
)

var (
	errEmptyURL         = errors.New("empty url")
	errInvalidURLScheme = errors.New("invalid url scheme")
	errInvalidHost      = errors.New("invalid host name")
	errInavlidPort      = errors.New("invalid port")
)

// Connection struct
type Connection struct {
	Conn   io.ReadWriteCloser
	writer writer
	closed bool
	mu     sync.RWMutex
}

func validateURL(uri string) (*url.URL, error) {
	if len(uri) == 0 {
		return nil, errEmptyURL
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "tcp" {
		return nil, errInvalidURLScheme
	}

	if u.Hostname() == "" {
		return nil, errInvalidHost
	}

	if u.Port() != "19000" {
		return nil, errInavlidPort
	}

	return u, nil
}

// Dial retuns a pointer to the connection struct
func Dial(url string) (*Connection, error) {
	return dial(url)
}

func dial(url string) (*Connection, error) {
	u, err := validateURL(url)
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(u.Hostname(), u.Port())
	conn, err := net.DialTimeout(u.Scheme, addr, tcpTimeout)
	if err != nil {
		return nil, err
	}
	fmt.Println("Successfully connected to CAN interface")

	return open(conn), nil
}

func open(conn io.ReadWriteCloser) *Connection {
	c := &Connection{
		Conn:   conn,
		writer: writer{w: bufio.NewWriter(conn)},
	}

	return c
}

func (c *Connection) close() error {
	if err := c.Conn.Close(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true
	return nil
}

func (c *Connection) isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.closed
}
