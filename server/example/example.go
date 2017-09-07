package main

import (
	"log"

	"github.com/zhengxiaoyao0716/zcli/server"
)

func main() {
	if err := server.Start("Example", "127.0.0.1:4000"); err != nil {
		log.Fatalln(err)
	}
	for {
	}
}
