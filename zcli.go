package main

import (
	"flag"
	"log"

	"github.com/zhengxiaoyao0716/zcli/client"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	address := flag.String("addr", "", "Service address witch to be connected.")
	flag.Parse()

	if err := client.Start(*address); err != nil {
		log.Fatalln(err)
	}
}
