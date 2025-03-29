package services

import (
	"Kygram/proto/protopb"
	"Kygram/repository"
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type KeyExchangeService struct {
	repo *repository.ChatRepository
	protopb.UnimplementedKeyExchangeServiceServer
	publicKeys map[string]string
	mu         sync.Mutex
}

func NewKeyExchangeService(repo *repository.ChatRepository) *KeyExchangeService {
	return &KeyExchangeService{
		repo:       repo,
		publicKeys: make(map[string]string),
	}
}

func (s *KeyExchangeService) SendPublicKey(ctx context.Context, req *protopb.SendPublicKeyRequest) (*protopb.SendPublicKeyResponse, error) {
	userID, err := uuid.Parse(req.ClientId)
	if err != nil {
		return nil, fmt.Errorf("invalid client ID: %w", err)
	}

	err = s.repo.SavePublicKey(ctx, userID, req.PublicKey)
	if err != nil {
		return &protopb.SendPublicKeyResponse{Success: false, Error: err.Error()}, nil
	}
	return &protopb.SendPublicKeyResponse{Success: true}, nil
}

func (s *KeyExchangeService) ExchangeKeys(ctx context.Context, req *protopb.KeyExchangeRequest) (*protopb.KeyExchangeResponse, error) {
	chatID, err := uuid.Parse(req.ChatId)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	publicKeys, err := s.repo.GetPublicKeysByChatID(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get public keys: %w", err)
	}

	var clientPublicKeys []*protopb.ClientPublicKey
	for userID, publicKey := range publicKeys {
		clientPublicKeys = append(clientPublicKeys, &protopb.ClientPublicKey{
			ClientId:  userID.String(),
			PublicKey: publicKey,
		})
	}

	return &protopb.KeyExchangeResponse{PublicKeys: clientPublicKeys}, nil
}
