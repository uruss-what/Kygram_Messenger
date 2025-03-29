package services

import (
	"Kygram/config"
	"Kygram/models"
	"context"
	"fmt"
	"sync"
	"time"

	"Kygram/algos"
	"Kygram/proto/protopb"
	"Kygram/repository"
	"log"

	"github.com/streadway/amqp"

	"github.com/google/uuid"
)

type ChatService struct {
	protopb.UnimplementedChatServiceServer
	userRepo *repository.UserRepository
	chatRepo *repository.ChatRepository
	Broker   *MessageBroker
	streams  map[string]map[uuid.UUID]chan *protopb.Message
	mu       sync.Mutex
}

type MessageBroker struct {
	Conn *amqp.Connection
}

func (mb *MessageBroker) GetChannel() (*amqp.Channel, error) {
	return mb.Conn.Channel()
}

func NewChatService(userRepo *repository.UserRepository, chatRepo *repository.ChatRepository) *ChatService {
	broker, err := NewMessageBroker()
	if err != nil {
		log.Println("failed to initialize message broker:", err)
	}
	return &ChatService{
		userRepo: userRepo,
		chatRepo: chatRepo,
		Broker:   broker,
		streams:  make(map[string]map[uuid.UUID]chan *protopb.Message),
	}
}
func NewMessageBroker() (*MessageBroker, error) {
	conn, err := config.GetRabbitMQ()
	if err != nil {
		return nil, err
	}
	return &MessageBroker{Conn: conn}, nil
}

func (s *ChatService) ListUsers(ctx context.Context, req *protopb.ListUsersRequest) (*protopb.ListUsersResponse, error) {
	users, err := s.userRepo.ListUsers()
	if err != nil {
		return nil, err
	}

	var userResponses []*protopb.GetUserResponse
	for _, username := range users {
		userResponses = append(userResponses, &protopb.GetUserResponse{
			Username: username,
		})
	}

	return &protopb.ListUsersResponse{Users: userResponses}, nil
}

func (s *ChatService) CreateChat(ctx context.Context, req *protopb.CreateChatRequest) (*protopb.CreateChatResponse, error) {
	chatID := uuid.New()

	prime, err := algos.GeneratePrime(2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate prime number: %w", err)
	}

	creatorIDStr, ok := ctx.Value("user_id").(string)
	if !ok {
		return nil, fmt.Errorf("creator ID not found in context")
	}

	creatorID, err := uuid.Parse(creatorIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid creator ID: %w", err)
	}

	var participants []uuid.UUID
	for _, username := range req.Participants {
		userID, err := s.userRepo.GetUserIDByUsername(ctx, username)
		if err != nil {
			return nil, fmt.Errorf("failed to get user ID for username %s: %w", username, err)
		}
		participants = append(participants, userID)
	}

	participants = append(participants, creatorID)

	err = s.chatRepo.CreateChat(ctx, chatID, req.Name, req.Algorithm, req.Mode, req.Padding, prime.String(), participants)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	return &protopb.CreateChatResponse{
		ChatId: chatID.String(),
	}, nil
}

func (s *ChatService) PublishMessage(chatID uuid.UUID, message []byte) error {
	ch, err := s.Broker.Conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		chatID.String(),
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	err = ch.Publish(
		"",
		q.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/octet-stream",
			Body:        message,
		})
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	log.Printf("Message published to queue: %s", q.Name)
	return nil
}

func (s *ChatService) CloseChat(ctx context.Context, req *protopb.CloseChatRequest) (*protopb.CloseChatResponse, error) {
	chatID, err := uuid.Parse(req.ChatId)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	err = s.chatRepo.DeleteChat(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete chat: %w", err)
	}

	return &protopb.CloseChatResponse{
		Success: true,
	}, nil
}

func (s *ChatService) GetMessages(ctx context.Context, chatID uuid.UUID) ([]models.Message, error) {
	messages, err := s.chatRepo.GetMessages(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	for i, msg := range messages {
		username, err := s.userRepo.GetUsernameByID(ctx, msg.SenderID)
		if err != nil {
			return nil, fmt.Errorf("failed to get username for user %s: %w", msg.SenderID, err)
		}
		messages[i].SenderName = username
	}
	return messages, nil
}

func (s *ChatService) GetUsernameByID(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.userRepo.GetUsernameByID(ctx, userID)
}

func (s *ChatService) SendMessage(ctx context.Context, req *protopb.SendMessageRequest) (*protopb.SendMessageResponse, error) {
	chatID, err := uuid.Parse(req.ChatId)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	senderID, err := uuid.Parse(req.Sender)
	if err != nil {
		return nil, fmt.Errorf("invalid sender ID: %w", err)
	}

	exists, err := s.ChatExists(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to check chat existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("chat does not exist")
	}

	chat, err := s.chatRepo.GetChatByID(chatID)
	if err != nil {
		return nil, fmt.Errorf("chat not found: %w", err)
	}

	cipher, err := initCipher(chat.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("cipher init failed: %w", err)
	}

	mode, padding, err := validateEncryptionParams(chat.Mode, chat.Padding)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption params: %w", err)
	}

	encrypted, err := encryptMessage(cipher, mode, padding, []byte(req.Message))
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	msg := models.Message{
		MessageID:        uuid.New(),
		ChatID:           chatID,
		SenderID:         senderID,
		EncryptedMessage: encrypted,
		CreatedAt:        time.Now(),
		MessageType:      req.MessageType,
		FileName:         req.FileName,
		ChunkIndex:       int(req.ChunkIndex),
		TotalChunks:      int(req.TotalChunks),
	}

	if err := s.chatRepo.SaveMessage(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}

	if err := s.PublishMessage(chatID, encrypted); err != nil {
		return nil, fmt.Errorf("failed to publish message: %w", err)
	}

	return &protopb.SendMessageResponse{
		Success: true,
		Message: "Message sent successfully",
	}, nil
}

func initCipher(algorithm string) (algos.Cipher, error) {
	switch algorithm {
	case "twofish":
		return algos.NewTwofish()
	case "rc5":
		return algos.NewRC5()
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}

func validateEncryptionParams(modeStr, paddingStr string) (algos.EncryptionMode, algos.PaddingMode, error) {
	modes := map[string]algos.EncryptionMode{
		"ECB":         algos.ECB,
		"CBC":         algos.CBC,
		"PCBC":        algos.PCBC,
		"CFB":         algos.CFB,
		"OFB":         algos.OFB,
		"CTR":         algos.CTR,
		"RandomDelta": algos.RandomDelta,
	}

	paddings := map[string]algos.PaddingMode{
		"Zeros":    algos.Zeros,
		"ANSIX923": algos.ANSI_X_923,
		"PKCS7":    algos.PKCS7,
		"ISO10126": algos.ISO_10126,
	}

	mode, ok := modes[modeStr]
	if !ok {
		return 0, 0, fmt.Errorf("unsupported mode: %s", modeStr)
	}

	padding, ok := paddings[paddingStr]
	if !ok {
		return 0, 0, fmt.Errorf("unsupported padding: %s", paddingStr)
	}

	return mode, padding, nil
}

func (s *ChatService) EncryptMessage(msg *protopb.Message, customKey []byte) ([]byte, error) {
	cipher, err := initCipher(msg.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("cipher init failed: %w", err)
	}

	mode, padding, err := validateEncryptionParams(msg.Mode, msg.Padding)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption params: %w", err)
	}
	if customKey != nil {
		log.Printf("[ENCRYPTION] Шифрование сообщения с кастомным ключом длиной %d байт", len(customKey))
	} else {
		log.Printf("[ENCRYPTION] Шифрование сообщения со стандартным ключом")
	}
	key := customKey
	if key == nil {
		// для обратной совместимости используем старый фиксированный ключ
		key = []byte("securekey12345678")
	}

	iv := []byte("iv1234567890abcd") //todo

	expander, ok := cipher.(algos.KeyExpander)
	if !ok {
		return nil, fmt.Errorf("cipher doesn't support key expansion")
	}

	ctxEnc := algos.NewEncryptionContext(
		key,
		mode,
		padding,
		iv,
		cipher,
		expander,
	)

	return ctxEnc.Encrypt(msg.EncryptedMessage)
}

func encryptMessage(cipher algos.Cipher, mode algos.EncryptionMode, padding algos.PaddingMode, message []byte) ([]byte, error) {
	key := []byte("securekey12345678")
	iv := []byte("iv1234567890abcd")

	expander, ok := cipher.(algos.KeyExpander)
	if !ok {
		return nil, fmt.Errorf("cipher doesn't support key expansion")
	}

	ctxEnc := algos.NewEncryptionContext(
		key,
		mode,
		padding,
		iv,
		cipher,
		expander,
	)

	return ctxEnc.Encrypt(message)
}

func (s *ChatService) DecryptMessage(encryptedMsg []byte, algorithm, modeStr, paddingStr string, customKey []byte) ([]byte, error) {
	cipher, err := initCipher(algorithm)
	if err != nil {
		return nil, fmt.Errorf("cipher init failed: %w", err)
	}

	mode, padding, err := validateEncryptionParams(modeStr, paddingStr)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption params: %w", err)
	}

	key := customKey
	if key == nil {
		key = []byte("securekey12345678")
	}

	iv := []byte("iv1234567890abcd")

	expander, ok := cipher.(algos.KeyExpander)
	if !ok {
		return nil, fmt.Errorf("cipher doesn't support key expansion")
	}

	ctxDec := algos.NewEncryptionContext(
		key,
		mode,
		padding,
		iv,
		cipher,
		expander,
	)

	return ctxDec.Decrypt(encryptedMsg)
}

func (s *ChatService) GetChatHistory(ctx context.Context, req *protopb.GetChatHistoryRequest) (*protopb.GetChatHistoryResponse, error) {
	chatID, err := uuid.Parse(req.ChatId)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	messages, err := s.chatRepo.GetMessages(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	var messageResponses []*protopb.Message
	for _, msg := range messages {

		messageResponses = append(messageResponses, &protopb.Message{
			MessageId:        msg.MessageID.String(),
			ChatId:           msg.ChatID.String(),
			SenderId:         msg.SenderID.String(),
			SenderName:       msg.SenderName,
			EncryptedMessage: msg.EncryptedMessage,
			CreatedAt:        msg.CreatedAt.UTC().Format(time.RFC3339),
			MessageType:      msg.MessageType,
			FileName:         msg.FileName,
			ChunkIndex:       int32(msg.ChunkIndex),
			TotalChunks:      int32(msg.TotalChunks),
		})
	}

	var messageRecords []*protopb.MessageRecord
	for _, msg := range messageResponses {
		senderID, err := uuid.Parse(msg.SenderId)
		if err != nil {
			return nil, fmt.Errorf("invalid chat ID: %w", err)
		}
		username, err := s.userRepo.GetUsernameByID(ctx, senderID)
		if err != nil {
			return nil, fmt.Errorf("failed to get username for user %s: %w", senderID, err)
		}

		messageRecords = append(messageRecords, &protopb.MessageRecord{
			SenderId:         msg.SenderId,
			SenderName:       username,
			EncryptedMessage: msg.EncryptedMessage,
			CreatedAt:        msg.CreatedAt,
		})
	}

	return &protopb.GetChatHistoryResponse{
		Messages: messageRecords,
	}, nil
}

func (s *ChatService) ChatExists(ctx context.Context, chatID uuid.UUID) (bool, error) {
	exists, err := s.chatRepo.ChatExists(ctx, chatID)
	if err != nil {
		return false, fmt.Errorf("failed to check chat existence: %w", err)
	}
	return exists, nil
}
func (s *ChatService) StreamMessages(stream protopb.ChatService_StreamMessagesServer) error {
	fileChunks := make(map[string][][]byte)
	msg, err := stream.Recv()
	if err != nil {
		return err
	}

	chatID := msg.ChatId
	userID, err := uuid.Parse(msg.SenderId)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if _, ok := s.streams[chatID]; !ok {
		s.streams[chatID] = make(map[uuid.UUID]chan *protopb.Message)
	}
	ch := make(chan *protopb.Message, 100)
	s.streams[chatID][userID] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.streams[chatID], userID)
		if len(s.streams[chatID]) == 0 {
			delete(s.streams, chatID)
		}
		s.mu.Unlock()
	}()

	go func() {
		for message := range ch {
			if err := stream.Send(message); err != nil {
				log.Printf("Failed to send message to user %s: %v", userID, err)
				return
			}
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		if msg.MessageType == "file" {
			fileName := msg.FileName
			chunkIndex := int(msg.ChunkIndex)
			totalChunks := int(msg.TotalChunks)

			if fileChunks[fileName] == nil {
				fileChunks[fileName] = make([][]byte, totalChunks)
			}
			fileChunks[fileName][chunkIndex] = msg.EncryptedMessage

			if chunkIndex == totalChunks-1 {
				var fileData []byte
				for _, chunk := range fileChunks[fileName] {
					fileData = append(fileData, chunk...)
				}

				delete(fileChunks, fileName)
			}
		}

		message := models.Message{
			MessageID:        uuid.New(),
			ChatID:           uuid.MustParse(msg.ChatId),
			SenderID:         uuid.MustParse(msg.SenderId),
			EncryptedMessage: msg.EncryptedMessage,
			CreatedAt:        time.Now(),
			MessageType:      msg.MessageType,
			FileName:         msg.FileName,
			ChunkIndex:       int(msg.ChunkIndex),
			TotalChunks:      int(msg.TotalChunks),
		}
		if err := s.chatRepo.SaveMessage(context.Background(), message); err != nil {
			log.Printf("Failed to save message: %v", err)
			continue
		}

		s.mu.Lock()
		for _, userChan := range s.streams[chatID] {
			userChan <- msg
		}
		s.mu.Unlock()
	}
}
