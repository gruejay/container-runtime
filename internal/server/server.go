package server

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type RuntimeService struct {
	runtimeapi.UnimplementedRuntimeServiceServer
}

func (r *RuntimeService) Version(context.Context, *runtimeapi.VersionRequest) (*runtimeapi.VersionResponse, error) {
	return &runtimeapi.VersionResponse{
		Version:           "v1",
		RuntimeName:       "boxr",
		RuntimeVersion:    "0.0.1",
		RuntimeApiVersion: "v1",
	}, nil
}

func Serve() {
	socketPath := "/tmp/boxr.sock"
	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	runtimeService := RuntimeService{}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	reflection.Register(grpcServer)
	runtimeapi.RegisterRuntimeServiceServer(grpcServer, &runtimeService)
	grpcServer.Serve(lis)
}
