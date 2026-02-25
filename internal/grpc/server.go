package grpc

import (
	"context"
	"log/slog"
	"net"

	"github.com/conorfennell/knolhash/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	pb.UnimplementedGreeterServer
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	slog.Info("Received gRPC SayHello request", "name", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

func StartServer(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, NewServer())
	reflection.Register(s)
	slog.Info("Starting gRPC server", "addr", addr)
	return s.Serve(lis)
}
