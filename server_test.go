package boptest

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/jamesryancoleman/bos/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestStartServer(t *testing.T) {
	// create boptest test case
	testCase, err := NewTestCase(testcase,
		WithStartTime(3600*24*31),
		WithStep(2),              // seconds
		WithHost("0.0.0.0:1025"), // the boptest docker container
	)
	if err != nil {
		fileLog.Error(err.Error())
	}
	defer testCase.Stop()

	s := NewServer("0.0.0.0:50070", testCase)
	s.Start()

	time.Sleep(time.Second)
}

func TestGet(t *testing.T) {
	// points to get
	var points = []string{
		"zon_reaTRooAir_y",
		"con_oveTSetCoo_u",
		"fcu_oveFan_activate",
	}

	// create boptest test case
	testCase, err := NewTestCase(testcase,
		WithStartTime(3600*24*31),
		WithStep(2),              // seconds
		WithHost("0.0.0.0:1025"), // the boptest docker container
	)
	if err != nil {
		fileLog.Error(err.Error())
	}
	defer testCase.Stop()

	s := NewServer("0.0.0.0:50070", testCase)
	s.Start()

	// query the server and ask for the points
	// set up connection to server
	conn, err := grpc.NewClient(s.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect >> %s", err.Error())
	}
	defer conn.Close()
	c := common.NewDeviceControlClient(conn)

	// issue Get rpc
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	r, err := c.Get(ctx, &common.GetRequest{
		Header: &common.Header{Src: "test.local", Dst: s.Addr},
		Keys:   points})
	if err != nil {
		fmt.Println(err.Error())
		t.Fail()
	}
	for i, p := range r.GetPairs() {
		if p.GetError() > 0 {
			fmt.Printf("pair %d: error %d '%s'\n", i, p.GetError(), p.GetErrorMsg())
		} else {
			fmt.Printf("pair %d: %v\n", i, p)
		}
	}

}
