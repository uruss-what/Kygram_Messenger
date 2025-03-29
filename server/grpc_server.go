package server

import (
	"fmt"
	"log"
	"net"

	//"Kygram/config"
	"Kygram/config"
	"Kygram/proto/protopb"
	"Kygram/repository"
	"Kygram/services"

	"google.golang.org/grpc"
)

func StartGRPCServer() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	userRepo := repository.NewUserRepository()
	chatRepo := repository.NewChatRepository(config.GetDB())
	secretKey := []byte("qoiewpqvnfj")

	authService := services.NewAuthService(userRepo, secretKey)
	keyExchangeService := services.NewKeyExchangeService(chatRepo)
	chatService := services.NewChatService(userRepo, chatRepo)

	grpcServer := grpc.NewServer()
	protopb.RegisterUserServiceServer(grpcServer, authService)
	protopb.RegisterKeyExchangeServiceServer(grpcServer, keyExchangeService)
	protopb.RegisterChatServiceServer(grpcServer, chatService)

	fmt.Println("gRPC server is running on port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
