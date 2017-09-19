package main

import (
	"log"
	"os"

	"github.com/zhengxiaoyao0716/zcli/client"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	client.Start(os.Args[1:])
}
