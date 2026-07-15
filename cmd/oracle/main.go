// MobileChain 链下预言机签名服务（独立二进制入口）。
// 实际逻辑见 internal/oraclesvc（mcchaind oracle 子命令复用同一实现）。
package main

import (
	"log"
	"os"

	"mcchain/internal/oraclesvc"
)

func main() {
	listen := os.Getenv("ORACLE_LISTEN")
	if listen == "" {
		listen = ":8080"
	}
	if err := oraclesvc.Run(listen); err != nil {
		log.Fatal(err)
	}
}
