package client

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/zhengxiaoyao0716/util/cout"

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
func Start(args []string) {
	address := flag.String("addr", "", "Service address witch to be connected.")
	cmds := flag.String("c", "", "Commands that would send to server directly.")
	flag.CommandLine.Parse(args)

	network := "tcp"
	if strings.HasSuffix(*address, ".sock") {
		network = "unix"
	}
	conn, err := net.Dial(network, *address)
	if err != nil {
		log.Fatalln(err)
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
		log.Fatalln(err)
	}
	for check := ""; check != "sync"; check = <-wait {
	}

	if *cmds != "" {
		cout.Print(console.In)
		for _, cmd := range strings.Split(*cmds, ";") {
			console.Log(cmd)
			c.Send("/usr/cmd", cmd)
			cout.Print(<-wait) // echo all middle step result.
		}
		console.Log(cout.Warn("^C"))
		return
	}

	console.CatchInterrupt(func() { c.Send("/sys/close", "") })

	c.Send("/usr/cmd", "--help")
	tip := <-wait
	for {
		cmds := console.ReadLine(tip)
		for _, cmd := range strings.Split(cmds, ";") {
			if err := c.Send("/usr/cmd", cmd); err != nil {
				log.Println(err)
			}
			tip = <-wait // middle step result would be silence.
		}
	}
}
