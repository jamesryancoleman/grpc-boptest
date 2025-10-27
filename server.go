package boptest

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/jamesryancoleman/bos/common"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type serverOption func(*Server)

// seconds since start of year
func WithAddr(addr string) serverOption {
	return func(s *Server) {
		s.Addr = addr
	}
}

type Server struct {
	common.UnimplementedDeviceControlServer

	Addr     string
	TestCase *TestCase
}

func NewServer(listenAddr string, testCase *TestCase, opts ...serverOption) *Server {
	var s Server
	s.Addr = listenAddr
	// apply optional parameters
	for _, opt := range opts {
		opt(&s)
	}

	s.TestCase = testCase
	return &s
}

func (s *Server) Start() error {
	// start the simulation up the simulation
	err := s.TestCase.Start()
	if err != nil {
		FileLog.Error(err.Error())
		return err
	}

	// server set up
	lis, err := net.Listen("tcp", s.Addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		return err
	}

	// create the grpc server
	server := grpc.NewServer()
	common.RegisterDeviceControlServer(server, s)

	// start the blocking gRPC server in a go routine
	go func() {
		if err := server.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// log successs
	common.TermLog.Info("server started", "listenAddr", s.Addr)
	common.FileLog.Info("server started", "listen_addr", s.Addr)

	return nil
}

func (s *Server) Get(ctx context.Context, req *common.GetRequest) (*common.GetResponse, error) {
	header := req.GetHeader()
	header.Dst = header.GetSrc()
	header.Src = header.GetDst()

	// set the header time based on the state machine
	t, err := s.TestCase.State.Time()
	fmt.Printf("simulation time %s\n", t.Format(time.RFC3339))
	if err != nil {
		TermLog.Warn("no time in State map")
		t = time.Now()
	}
	header.Time = timestamppb.New(t)

	// extract keys from uri strings
	keys := req.GetKeys()
	TermLog.Info(fmt.Sprintf("received keys: %v", keys))
	var points []string
	pointToUri := make(map[string]string, len(keys))
	for _, k := range keys {
		p := schemaRe.FindStringSubmatch(k)[2]
		points = append(points, p)
		pointToUri[p] = k
	}

	// get the state from the simulation
	m := s.TestCase.State.GetMultiple(points)

	// package the results
	pairs := make([]*common.GetPair, len(points))
	var i int
	for p, v := range m {
		if u, ok := pointToUri[p]; ok {
			pairs[i] = &common.GetPair{
				Key:   u,
				Value: fmt.Sprintf("%v", v),
				Time:  timestamppb.New(t),
			}
		}
		i++
	}

	fmt.Printf("header time %s\n", header.Time.AsTime().Format(time.RFC3339))

	// fetch values from simulation
	return &common.GetResponse{
		Header: header,
		Pairs:  pairs,
	}, nil
}

func (s *Server) Set(ctx context.Context, req *common.SetRequest) (*common.SetResponse, error) {
	header := req.GetHeader()
	header.Dst = header.GetSrc()
	header.Src = header.GetDst()

	// TODO: confirm if setting a time is necessary

	// extract keys, convert to internal name, and reassociate with values
	pairs := req.GetPairs()
	TermLog.Info("set request received", "num_pairs", len(pairs))

	// create a look up table to convert external to internal keys
	externalToInternal := make(map[string]string, len(pairs))

	payload := make(map[string]string, len(pairs))
	for i := range pairs {
		external := pairs[i].GetKey()
		internal := schemaRe.FindStringSubmatch(external)[2]
		payload[internal] = pairs[i].GetValue()
		externalToInternal[external] = internal

		// write to the simulation
		s.TestCase.SetInput(internal, pairs[i].GetValue())
	}

	// return
	return &common.SetResponse{
		Header: header,
		Pairs:  pairs,
	}, nil
}

// Any call must confirm the testcase is actually running and if not, start it.
// TODO: determine what time to start it at.

// func (s *Server) Get(ctx context.Context, req *common.GetRequest) (*common.GetResponse, error) {
// 	keys := req.GetKeys()
// 	pairs := make([]*common.GetPair, len(keys))

// 	// fetch values
// 	for i, k := range keys {
// 		tc, err := ParseTerabee(k)
// 		if err != nil {
// 			errMsg := err.Error()
// 			common.Lwarn.Println(errMsg)
// 			pairs[i] = &common.GetPair{
// 				Error:    common.GetError_GET_ERROR_UNSPECIFIED.Enum(),
// 				ErrorMsg: &errMsg,
// 			}
// 			continue
// 		}
// 		occupancy, err := tc.GetNetOccupancy()
// 		if err != nil {
// 			errMsg := err.Error()
// 			pairs[i] = &common.GetPair{
// 				Error:    common.GetError_GET_ERROR_UNSPECIFIED.Enum(),
// 				ErrorMsg: &errMsg,
// 			}
// 			continue
// 		}
// 		pairs[i] = &common.GetPair{Key: k, Value: fmt.Sprint(occupancy)}
// 	}
// 	return &common.GetResponse{Pairs: pairs}, nil
// }
