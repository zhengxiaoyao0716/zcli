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

	"github.com/zhengxiaoyao0716/util/console"

	"github.com/zhengxiaoyao0716/util/cout"
)

var (
	// BufSize .
	BufSize int
)

// Mode [UNUSED:000][UNUSED:000][GUEST:000][USER:000][ROOT:000], 000:rwx, r:read, w:write, x:exec,
type Mode int16

// Emulate of Mode
const (
	ModeRoot Mode = 07 << (3 * iota)
	ModeUser
	ModeGuest

	ModeRx Mode = 01 << (iota - 3)
	ModeRw
	ModeRr
	ModeUx
	ModeUw
	ModeUr
	ModeGx
	ModeGw
	ModeGr

	ModeAll = 0777 // All identities, include Guest, User, Root
	ModeReg = 0077 // Only identities who registered, User or Root
	ModeBan = 0000 // Identity who has nothing access permission.
)

// Conn .
type Conn struct {
	net.Conn
	mode *Mode
	id   int
	time string
}

// GetMode .
func (c *Conn) GetMode(ms ...Mode) Mode {
	mode := *c.mode
	for _, m := range ms {
		mode = mode & m
	}
	return mode
}

// SetMode .
func (c *Conn) SetMode(mode Mode, ms ...Mode) {
	for _, m := range ms {
		mode = mode & m
	}
	*c.mode = mode
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
	mode := ModeGuest
	c := Conn{conn, &mode, id, time.Now().String()[0:19]}
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
func Status(marks ...Conn) string {
	var sorted connSlice
	for _, c := range conns {
		sorted = append(sorted, c)
	}
	sort.Sort(sorted)

	ls := []string{
		fmt.Sprintf("--%+3s---%-25s---%-19s---%-5s--", strings.Repeat("-", 3), strings.Repeat("-", 25), strings.Repeat("-", 19), strings.Repeat("-", 5)),
		fmt.Sprintf("| %+3s | %-25s | %-19s | %-5s |", "ID", "ADDRESS", "TIME", "MODE"),
		fmt.Sprintf("| %+3s---%-25s---%-19s---%-5s |", strings.Repeat("-", 3), strings.Repeat("-", 25), strings.Repeat("-", 19), strings.Repeat("-", 5)),
	}
	markSet := map[int]bool{}
	for _, c := range marks {
		markSet[c.id] = true
	}
	for _, c := range sorted {
		if _, ok := markSet[c.id]; ok {
			ls = append(ls, fmt.Sprintf(
				console.In+"%s | %s | %s | %s |",
				cout.Yes("%3d", c.id),
				cout.Yes("%-25s", c.RemoteAddr()),
				cout.Yes(c.time),
				cout.Yes("%05o", c.mode)),
			)
		} else {
			ls = append(ls, fmt.Sprintf("| %3d | %s | %s | %05o |", c.id, cout.Info("%-25s", c.RemoteAddr()), c.time, c.mode))
		}
	}
	ls = append(ls, ls[0])
	return strings.Join(ls, "\n")
}

func init() {
	if BufSize == 0 {
		BufSize = 511
	}
}
