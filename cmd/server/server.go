package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jamesryancoleman/bos/drivers/boptest"
)

const defaultAddr string = "0.0.0.0:50066"

func main() {
	caseNamePtr := flag.String("case", "bestest_air", "name of the testcase")
	startTimePtr := flag.Int("start", 2678400, "seconds since start of year")
	stepPtr := flag.Int("step", 60, "seconds to advance the simluation per update")

	flag.Parse()

	// create boptest test case
	testCase, err := boptest.NewTestCase(
		*caseNamePtr,
		boptest.WithHost("0.0.0.0:1025"), // the boptest docker container
		boptest.WithStartTime(*startTimePtr),
		boptest.WithStep(*stepPtr),
		boptest.WithStartNow(),
	)
	if err != nil {
		boptest.FileLog.Error(err.Error())
	}
	defer testCase.Stop()

	s := boptest.NewServer(defaultAddr, testCase)
	s.Start()

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("server running, press ctrl+c to exit...")
	<-done // Will block here until user hits ctrl+c
	fmt.Println("\nshutting down.")
}
