package client

import (
	"context"
	"log"
	"math/big"
	"time"

	"Kygram/algos"
	"Kygram/proto/protopb"

	"google.golang.org/grpc"
)

type KeyExchangeClient struct {
	client protopb.KeyExchangeServiceClient
}

func NewKeyExchangeClient(serverAddr string) (*KeyExchangeClient, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	client := protopb.NewKeyExchangeServiceClient(conn)
	return &KeyExchangeClient{client: client}, nil
}

func (c *KeyExchangeClient) GenerateAndSendKey(chatID string, userID string) (*big.Int, error) {
	prime, err := algos.GeneratePrime(1024)
	if err != nil {
		return nil, err
	}

	privateKey, err := algos.GeneratePrivateKey(prime)
	if err != nil {
		return nil, err
	}

	generator := big.NewInt(2)
	publicKey := algos.GeneratePublicKey(generator, privateKey, prime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = c.client.SendPublicKey(ctx, &protopb.SendPublicKeyRequest{
		ChatId:    chatID,
		ClientId:  userID,
		PublicKey: publicKey.String(),
	})
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func (c *KeyExchangeClient) GetPeerKeys(chatID string) ([]*protopb.ClientPublicKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.ExchangeKeys(ctx, &protopb.KeyExchangeRequest{
		ChatId: chatID,
	})
	if err != nil {
		return nil, err
	}

	return resp.PublicKeys, nil
}

func (c *KeyExchangeClient) ComputeSharedKeys(privateKey *big.Int, peerKeys []*protopb.ClientPublicKey, prime *big.Int) map[string][]byte {
	sharedKeys := make(map[string][]byte)

	for _, peer := range peerKeys {
		if peer.PublicKey == "" {
			log.Printf("Empty public key for client %s, skipping", peer.ClientId)
			continue
		}

		peerPublicKey, ok := new(big.Int).SetString(peer.PublicKey, 10)
		if !ok {
			log.Printf("Failed to parse public key for client %s", peer.ClientId)
			continue
		}

		sharedKey := algos.GenerateSharedKey(privateKey, peerPublicKey, prime)

		hashedKey := algos.HashSharedKey(sharedKey)

		sharedKeys[peer.ClientId] = hashedKey
	}
	log.Printf("[DH CLIENT] Успешно вычислены %d общих ключей", len(sharedKeys))
	return sharedKeys
}
