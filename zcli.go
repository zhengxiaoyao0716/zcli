package main

import (
	"flag"
	"log"

	"github.com/zhengxiaoyao0716/zcli/client"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	address := flag.String("addr", "", "Service address witch to be connected.")
	cmds := flag.String("c", "", "Commands that would send to server directly.")
	flag.Parse()

	if err := client.Start(*address, *cmds); err != nil {
		log.Fatalln(err)
	}
}
