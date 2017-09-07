package connect

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/zhengxiaoyao0716/util/cout"
)

var (
	// BufSize .
	BufSize int
)

// Conn .
type Conn struct {
	net.Conn
	time string
}

// Read .
func (c *Conn) Read() (string, error) {
	var bs []byte
	for {
		buf := make([]byte, BufSize+1)
		n, err := c.Conn.Read(buf)
		if err != nil {
			return "", err
		}

		bs = append(bs, buf[1:n]...)
		if buf[0] == 0xff {
			break
		}
	}
	return string(bs), nil
}

// Write .
func (c *Conn) Write(msg string) error {
	bs := append([]byte{0x00}, []byte(msg)...)
	for len(bs)-1 > BufSize {
		pack := bs[0 : BufSize+1]
		bs = bs[BufSize:]

		pack[0] = 0x00
		if _, err := c.Conn.Write(pack); err != nil {
			return err
		}
	}
	bs[0] = 0xff
	if _, err := c.Conn.Write(bs); err != nil {
		return err
	}
	return nil
}

// Receive .
func (c *Conn) Receive() (string, string, error) {
	msg, err := c.Read()
	if err != nil {
		return "", "", err
	}
	seps := strings.SplitN(msg, "\n", 2)
	var (
		path = seps[0]
		data string
	)
	if len(seps) == 2 {
		data = seps[1]
	}
	return path, data, nil
}

// Send .
func (c *Conn) Send(path string, data string) error { return c.Write(path + "\n" + data) }

// ErrClose used to stop the loop of `Server` function.
var ErrClose = errors.New("close")

// Server .
func (c *Conn) Server(handles map[string]func(data string) error) error {
	for {
		path, data, err := c.Receive()
		if err == io.EOF || path == "" {
			return nil
		}
		h, ok := handles[path]
		if !ok {
			return fmt.Errorf("missing handler, path=%s", path)
		}
		if err := h(data); err != nil {
			if err == ErrClose {
				// log.Println("connect was closed")
				return nil
			}
			return err
		}
	}
}

// New .
func New(c net.Conn) Conn {
	return Conn{c, time.Now().String()[0:19]}
}

func (c *Conn) String() string {
	return fmt.Sprintf("| %s | %s |", cout.Info("%-25s", c.RemoteAddr()), c.time)
}

func init() {
	if BufSize == 0 {
		BufSize = 511
	}
}
