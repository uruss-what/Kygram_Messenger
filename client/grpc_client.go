package client

import (
	"context"
	"log"

	"Kygram/proto/protopb"

	"google.golang.org/grpc"
)

type ChatClient struct {
	client protopb.ChatServiceClient
	stream protopb.ChatService_StreamMessagesClient
}

func NewChatClient(serverAddr string) (*ChatClient, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	client := protopb.NewChatServiceClient(conn)
	return &ChatClient{client: client}, nil
}

func (c *ChatClient) StreamMessages(chatID, userID string) error {
	stream, err := c.client.StreamMessages(context.Background())
	if err != nil {
		return err
	}

	err = stream.Send(&protopb.Message{
		ChatId:   chatID,
		SenderId: userID,
	})
	if err != nil {
		return err
	}

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				log.Printf("Failed to receive message: %v", err)
				return
			}
			log.Printf("Received message: %s", msg.EncryptedMessage)
		}
	}()

	return nil
}
