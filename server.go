package boptest

import (
	"log"
	"net"

	"github.com/jamesryancoleman/bos/common"
	"google.golang.org/grpc"
)

type Server struct {
	common.UnimplementedGetSetRunServer
	Addr string
}

func (s *Server) Start(listenAddr string) {
	s.Addr = listenAddr
	lis, err := net.Listen("tcp", s.Addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		return
	}

	// create the grpc server
	server := grpc.NewServer()
	common.RegisterGetSetRunServer(server, s)

	// log successs
	common.TermLog.Info("server started", "listenAddr", s.Addr)
	common.FileLog.Info("server started", "listen_addr", s.Addr)

	// start the blocking gRPC server in a go routine
	go func() {
		if err := server.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
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
