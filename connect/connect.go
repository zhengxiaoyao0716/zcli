package connect

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
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
	id   int
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

var (
	conns = map[int]Conn{}
	id    = 0
	// lock  = make(chan bool, 1)
	mutex sync.Mutex // witch is better?
)

// New .
func New(conn net.Conn) Conn {
	// lock <- true
	mutex.Lock()
	// defer func() { <-lock }()
	defer mutex.Unlock()

	if id > 2*len(conns) { // fill the vacant.
		id = 0
	}
	for _, ok := conns[id]; ok; _, ok = conns[id] {
		id++
	}
	c := Conn{conn, id, time.Now().String()[0:19]}
	conns[c.id] = c
	return c
}

// Close .
func (c Conn) Close() error {
	delete(conns, c.id)
	return c.Conn.Close()
}

// Get .
func Get(id int) (Conn, error) {
	c, ok := conns[id]
	if !ok {
		return c, errors.New("no conn found, id: " + cout.Err("%-3d", id))
	}
	return c, nil
}

type connSlice []Conn

func (p connSlice) Len() int           { return len(p) }
func (p connSlice) Less(i, j int) bool { return p[i].time > p[j].time }
func (p connSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Status .
func Status() string {
	var sorted connSlice
	for _, c := range conns {
		sorted = append(sorted, c)
	}
	sort.Sort(sorted)

	ls := []string{
		fmt.Sprintf("--%-3s---%-25s---%-19s--", strings.Repeat("-", 3), strings.Repeat("-", 25), strings.Repeat("-", 19)),
		fmt.Sprintf("| %-3s | %-25s | %-19s |", "ID", "ADDRESS", "TIME"),
		fmt.Sprintf("| %-3s---%-25s---%-19s |", strings.Repeat("-", 3), strings.Repeat("-", 25), strings.Repeat("-", 19)),
	}
	for _, c := range sorted {
		ls = append(ls, fmt.Sprintf("| %3d | %s | %s |", c.id, cout.Info("%-25s", c.RemoteAddr()), c.time))
	}
	ls = append(ls, ls[0])
	return strings.Join(ls, "\n")
}

func init() {
	if BufSize == 0 {
		BufSize = 511
	}
}
