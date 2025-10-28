package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	boptest "github.com/jamesryancoleman/grpc-boptest"
)

const defaultAddr string = "0.0.0.0:50066"

func main() {
	caseNamePtr := flag.String("case", "bestest_air", "name of the testcase")
	// Default is July 1st
	startTimePtr := flag.Int("start", 181*24*60*60, "seconds since start of year")
	stepPtr := flag.Int("step", 60, "seconds to advance the simluation per update")
	freqPtr := flag.Int("freq", 15, "update simlulation every SECONDS")

	flag.Parse()

	// create boptest test case
	testCase, err := boptest.NewTestCase(
		*caseNamePtr,
		boptest.WithHost("nuc.local:1025"), // the boptest docker container
		boptest.WithStartTime(*startTimePtr),
		boptest.WithStep(*stepPtr),
		boptest.WithUpdateFrequency(*freqPtr),
		boptest.WithStartNow(),
	)
	if err != nil {
		boptest.FileLog.Error(err.Error())
		boptest.TermLog.Error(err.Error())
	}
	defer testCase.Stop()

	s := boptest.NewServer(defaultAddr, testCase)
	s.Start()
	fmt.Printf("boptest server started @ %s\n", defaultAddr)

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("server running, press ctrl+c to exit...")
	<-done // Will block here until user hits ctrl+c
	fmt.Println("\nshutting down.")
}
