package main

import (
	"log"

	"github.com/zhengxiaoyao0716/util/console"
	"github.com/zhengxiaoyao0716/util/cout"
	"github.com/zhengxiaoyao0716/zcli/server"
)

func main() {
	addr := "127.0.0.1:4000"
	if err := server.Start("Example", addr); err != nil {
		log.Fatalln(err)
	}
	console.Log("Service start, use `zcli -addr %s` to connect it.", cout.Info(addr))
	for {
	}
}
