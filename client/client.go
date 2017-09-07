package client

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/zhengxiaoyao0716/util/console"

	"github.com/zhengxiaoyao0716/zcli/connect"
)

func reader(r io.Reader) {
	buf := make([]byte, 512)
	for {
		n, err := r.Read(buf[:])
		if err != nil {
			return
		}
		println("Client got:", string(buf[0:n]))
	}
}

// Start .
func Start(address string) error {
	network := "tcp"
	if strings.HasSuffix(address, ".sock") {
		network = "unix"
	}
	conn, err := net.Dial(network, address)
	if err != nil {
		return err
	}
	c := connect.New(conn)
	defer c.Close()

	wait := make(chan string, 1)
	go func() {
		if err := c.Server(map[string]func(string) error{
			"/sys/close": func(data string) error {
				fmt.Println("remote server has closed the connection:", data)
				return connect.ErrClose
			},
			"/sys/buf/size/sync": func(data string) error {
				size, err := strconv.Atoi(data)
				if err != nil {
					return err
				}
				connect.BufSize = int(size)
				wait <- "sync"
				return nil
			},
			"/sys/pong": func(data string) error {
				fmt.Println(data)
				return nil
			},
			"/usr/cmd": func(data string) error {
				wait <- data
				return nil
			},
		}); err != nil {
			log.Println(err)
			return
		}
	}()

	// sync buf size
	if err := c.Send("/sys/buf/size/sync", ""); err != nil {
		return err
	}
	for check := ""; check != "sync"; check = <-wait {
	}

	console.CatchInterrupt(func() { c.Send("/sys/close", "") })

	c.Send("/usr/cmd", "--help")
	for {
		tip := <-wait
		if err := c.Send("/usr/cmd", console.ReadLine(tip)); err != nil {
			log.Println(err)
		}
	}
}
