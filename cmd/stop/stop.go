package main

import (
	"flag"
	"fmt"

	boptest "github.com/jamesryancoleman/grpc-boptest"
)

func main() {
	flag.Parse()
	args := flag.Args()

	boptest.Host = "nuc.local:1025"
	err := boptest.StopTestCase(args[0])
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Printf("stopped testcase %s\n", args[0])
	}
}
