package boptest

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/jamesryancoleman/bos/common"
	"google.golang.org/grpc"
)

type serverOption func(*Server)

// seconds since start of year
func WithAddr(addr string) serverOption {
	return func(s *Server) {
		s.Addr = addr
	}
}

type Server struct {
	common.UnimplementedGetSetRunServer

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
		fileLog.Error(err.Error())
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
	common.RegisterGetSetRunServer(server, s)

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

	keys := req.GetKeys()
	pairs := make([]*common.GetPair, len(keys))

	m := s.TestCase.State.GetMultiple(keys)
	var i int
	for k, v := range m {
		pairs[i] = &common.GetPair{
			Key:   k,
			Value: fmt.Sprintf("%v", v),
		}
		i++
	}

	// fetch values from simulation
	return &common.GetResponse{
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
