package main

import (
	"log"

	"github.com/zhengxiaoyao0716/util/cout"
	"github.com/zhengxiaoyao0716/zcli/server"
)

func main() {
	addr := "127.0.0.1:4000"
	if err := server.Start("Example", addr); err != nil {
		log.Fatalln(err)
	}
	cout.Printf("Service start, use `%s` to connect it.\n", cout.Log("zcli -addr %s", cout.Info(addr)))
	for {
	}
}
