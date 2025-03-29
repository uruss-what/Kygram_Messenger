package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"Kygram/proto/protopb"
	"Kygram/repository"
	"Kygram/services"

	"Kygram/client"

	"github.com/google/uuid"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatHandlers struct {
	ChatService *services.ChatService
	ChatRepo    *repository.ChatRepository
	GrpcClient  protopb.ChatServiceClient
}

func NewChatHandlers(chatService *services.ChatService, chatRepo *repository.ChatRepository, grpcClient protopb.ChatServiceClient) *ChatHandlers {
	return &ChatHandlers{
		ChatService: chatService,
		ChatRepo:    chatRepo,
		GrpcClient:  grpcClient,
	}
}

func (h *ChatHandlers) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	resp, err := h.ChatService.ListUsers(context.Background(), &protopb.ListUsersRequest{})
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ChatHandlers) CreateChatHandler(w http.ResponseWriter, r *http.Request) {

	var req protopb.CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode request: %v", err)
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	creatorID := r.Header.Get("X-User-ID")
	if creatorID == "" {
		http.Error(w, `{"error": "User ID not provided"}`, http.StatusBadRequest)
		return
	}

	log.Printf("Received creatorID: %s", creatorID)
	ctx := context.WithValue(r.Context(), "user_id", creatorID)

	if req.ChatId == "" {
		req.ChatId = uuid.New().String()
	}

	if req.Algorithm == "" || req.Mode == "" || req.Padding == "" {
		http.Error(w, `{"error": "Missing required fields"}`, http.StatusBadRequest)
		return
	}

	log.Printf("Creating chat with ID: %s, Algorithm: %s, Mode: %s, Padding: %s, Participants: %v",
		req.ChatId, req.Algorithm, req.Mode, req.Padding, req.Participants)

	resp, err := h.ChatService.CreateChat(ctx, &req)
	if err != nil {
		log.Printf("Failed to create chat: %v", err)
		http.Error(w, `{"error": "Failed to create chat"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ChatHandlers) ListUserChatsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, `{"error": "User ID not provided"}`, http.StatusBadRequest)
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, `{"error": "Invalid user ID format"}`, http.StatusBadRequest)
		return
	}

	chats, err := h.ChatRepo.GetUserChats(userUUID)
	if err != nil {
		log.Printf("Failed to get user chats: %v", err)
		http.Error(w, `{"error": "Failed to get user chats"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"chats":   chats,
	})
}

func (h *ChatHandlers) SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received message request")
	var req protopb.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr == "" {
		sendJSONError(w, "User ID not provided", http.StatusBadRequest)
		return
	}

	req.Sender = userIDStr
	resp, err := h.ChatService.SendMessage(r.Context(), &req)
	if err != nil {
		log.Printf("Failed to send message: %v", err)
		sendJSONError(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func sendJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

func (h *ChatHandlers) GetChatHandler(w http.ResponseWriter, r *http.Request) {
	chatID := r.URL.Query().Get("id")
	if chatID == "" {
		http.Error(w, `{"error": "Chat ID not provided"}`, http.StatusBadRequest)
		return
	}

	chatUUID, err := uuid.Parse(chatID)
	if err != nil {
		http.Error(w, `{"error": "Invalid chat ID format"}`, http.StatusBadRequest)
		return
	}

	chat, err := h.ChatRepo.GetChatByID(chatUUID)
	if err != nil {
		http.Error(w, `{"error": "Chat not found"}`, http.StatusNotFound)
		return
	}

	participants, err := h.ChatRepo.GetChatParticipants(chatUUID)
	if err != nil {
		http.Error(w, `{"error": "Failed to get participants"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"chat": map[string]interface{}{
			"id":           chat.ChatID.String(),
			"name":         chat.Name,
			"algorithm":    chat.Algorithm,
			"mode":         chat.Mode,
			"padding":      chat.Padding,
			"prime":        chat.Prime,
			"created_at":   chat.CreatedAt,
			"participants": participants,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *ChatHandlers) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	chatIDStr := r.URL.Query().Get("chat_id")
	userIDStr := r.URL.Query().Get("user_id")
	if chatIDStr == "" || userIDStr == "" {
		log.Println("Missing chat_id or user_id param")
		return
	}

	log.Printf("WebSocket connection established for chat_id=%s, user_id=%s", chatIDStr, userIDStr)

	chatUUID, err := uuid.Parse(chatIDStr)
	if err != nil {
		log.Println("Invalid chat_id format:", err)
		return
	}

	chat, err := h.ChatRepo.GetChatByID(chatUUID)
	if err != nil {
		log.Println("Failed to get chat info:", err)
		return
	}

	keyExchangeClient, err := client.NewKeyExchangeClient("localhost:50051")
	if err != nil {
		log.Println("Failed to create key exchange client:", err)
		return
	}

	privateKey, err := keyExchangeClient.GenerateAndSendKey(chatIDStr, userIDStr)
	if err != nil {
		log.Println("Failed to generate and send key:", err)
		log.Println("Using default keys for encryption")
	}

	var peerKeys []*protopb.ClientPublicKey
	var sharedKeys map[string][]byte

	peerKeys, err = keyExchangeClient.GetPeerKeys(chatIDStr)
	if err != nil {
		log.Println("Failed to get peer keys:", err)
	} else {
		if prime, ok := new(big.Int).SetString(chat.Prime, 10); ok && privateKey != nil {
			sharedKeys = keyExchangeClient.ComputeSharedKeys(privateKey, peerKeys, prime)
		} else {
			log.Println("Failed to parse prime number or private key is nil, using default keys")
		}
	}

	stream, err := h.GrpcClient.StreamMessages(r.Context())
	if err != nil {
		log.Println("Failed to create gRPC stream:", err)
		return
	}
	defer stream.CloseSend()

	if err := stream.Send(&protopb.Message{
		ChatId:   chatIDStr,
		SenderId: userIDStr,
	}); err != nil {
		log.Println("Failed to send initial message:", err)
		return
	}

	fileChunks := make(map[string][][]byte)

	go func() {

		defer func() {
			if r := recover(); r != nil {
				log.Println("Recovered from panic in WebSocket handler:", r)
			}
		}()

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				log.Println("WebSocket read error:", err)
				return
			}
			var messageData map[string]interface{}
			if err := json.Unmarshal(msgBytes, &messageData); err != nil {
				log.Println("Failed to parse WebSocket message:", err)
				continue
			}

			log.Printf("Received message: %v", messageData)

			if messageData["type"] == "text" {
				messageText, ok := messageData["text"].(string)
				if !ok {
					log.Println("Invalid message format: missing 'text' field")
					continue
				}

				log.Printf("Processing text message: %s", messageText)

				var encryptionKey []byte
				if sharedKeys != nil {
					if recipientKey, ok := sharedKeys[userIDStr]; ok {
						encryptionKey = recipientKey
					}
				}

				encryptedMsg, err := h.ChatService.EncryptMessage(&protopb.Message{
					ChatId:           chatIDStr,
					SenderId:         userIDStr,
					EncryptedMessage: []byte(messageText),
					Algorithm:        chat.Algorithm,
					Mode:             chat.Mode,
					Padding:          chat.Padding,
				}, encryptionKey)
				if err != nil {
					log.Println("Failed to encrypt message:", err)
					continue
				}
				if err := stream.Send(&protopb.Message{
					MessageId:        uuid.New().String(),
					ChatId:           chatIDStr,
					SenderId:         userIDStr,
					EncryptedMessage: encryptedMsg,
					Algorithm:        chat.Algorithm,
					Mode:             chat.Mode,
					Padding:          chat.Padding,
					MessageType:      "text",
				}); err != nil {
					log.Println("Failed to send message via gRPC:", err)
					continue
				}

			}
			if messageData["type"] == "file" {
				fileName := messageData["file_name"].(string)
				chunkIndex := int(messageData["chunk_index"].(float64))
				totalChunks := int(messageData["total_chunks"].(float64))
				chunkData := messageData["data"].([]interface{})

				log.Printf("Processing file chunk: %s (chunk %d of %d)", fileName, chunkIndex+1, totalChunks)

				chunkBytes := make([]byte, len(chunkData))
				for i, v := range chunkData {
					chunkBytes[i] = byte(v.(float64))
				}

				if fileChunks[fileName] == nil {
					fileChunks[fileName] = make([][]byte, totalChunks)
				}
				fileChunks[fileName][chunkIndex] = chunkBytes

				log.Printf("Saved chunk %d for file %s", chunkIndex+1, fileName)

				if chunkIndex == totalChunks-1 {
					var fileData []byte
					for i, chunk := range fileChunks[fileName] {
						if chunk == nil {
							log.Printf("Missing chunk %d for file %s", i+1, fileName)
							return
						}
						fileData = append(fileData, chunk...)
					}

					log.Printf("All chunks received for file %s. Total size: %d bytes", fileName, len(fileData))

					var encryptionKey []byte
					if sharedKeys != nil {
						if recipientKey, ok := sharedKeys[userIDStr]; ok {
							encryptionKey = recipientKey
						}
					}

					encryptedFile, err := h.ChatService.EncryptMessage(&protopb.Message{
						ChatId:           chatIDStr,
						SenderId:         userIDStr,
						EncryptedMessage: fileData,
						Algorithm:        chat.Algorithm,
						Mode:             chat.Mode,
						Padding:          chat.Padding,
					}, encryptionKey)
					if err != nil {
						log.Println("Failed to encrypt file:", err)
						return
					}

					if err := stream.Send(&protopb.Message{
						ChatId:           chatIDStr,
						SenderId:         userIDStr,
						EncryptedMessage: encryptedFile,
						Algorithm:        chat.Algorithm,
						Mode:             chat.Mode,
						Padding:          chat.Padding,
						MessageType:      "file",
						FileName:         fileName,
						ChunkIndex:       int32(chunkIndex),
						TotalChunks:      int32(totalChunks),
					}); err != nil {
						log.Println("Failed to send file via gRPC:", err)
						return
					}

					delete(fileChunks, fileName)
				}
			}
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Println("Failed to receive message from gRPC stream:", err)
			return
		}

		var decryptionKey []byte
		if sharedKeys != nil {
			if senderKey, ok := sharedKeys[msg.SenderId]; ok {
				decryptionKey = senderKey
			}
		}

		decryptedMsg, err := h.ChatService.DecryptMessage(msg.EncryptedMessage, msg.Algorithm, msg.Mode, msg.Padding, decryptionKey)
		if err != nil {
			log.Println("Failed to decrypt message:", err)
			return
		}
		userUUID, err := uuid.Parse(msg.SenderId)
		if err != nil {
			log.Println("Invalid user_id format:", err)
			return
		}

		senderName, err := h.ChatService.GetUsernameByID(r.Context(), userUUID)
		if err != nil {
			log.Println("Failed to get sender name:", err)
			return
		}

		if msg.MessageType == "file" {
			decryptedFile, err := h.ChatService.DecryptMessage(msg.EncryptedMessage, msg.Algorithm, msg.Mode, msg.Padding, decryptionKey)
			if err != nil {
				log.Println("Failed to decrypt file:", err)
				return
			}

			log.Printf("Decrypted file size: %d bytes", len(decryptedFile))

			base64Data := base64.StdEncoding.EncodeToString(decryptedFile)

			response := map[string]interface{}{
				"sender_id":    userUUID,
				"sender_name":  senderName,
				"message":      base64Data,
				"created_at":   time.Now().Format(time.RFC3339),
				"message_type": "file",
				"file_name":    msg.FileName,
				"is_base64":    true,
			}

			if err := conn.WriteJSON(response); err != nil {
				log.Println("Failed to send file via WebSocket:", err)
				return
			}
		} else {
			response := map[string]interface{}{
				"sender_id":    userUUID,
				"sender_name":  senderName,
				"message":      string(decryptedMsg),
				"created_at":   time.Now().Format(time.RFC3339),
				"message_type": "text",
			}

			if err := conn.WriteJSON(response); err != nil {
				log.Println("Failed to send message via WebSocket:", err)
				return
			}
		}
	}
}

func (h *ChatHandlers) GetMessagesHandler(w http.ResponseWriter, r *http.Request) {
	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		sendJSONError(w, "Chat ID is required", http.StatusBadRequest)
		return
	}

	resp, err := h.ChatService.GetChatHistory(r.Context(), &protopb.GetChatHistoryRequest{
		ChatId: chatID,
	})
	if err != nil {
		sendJSONError(w, "Failed to get messages", http.StatusInternalServerError)
		return
	}

	chatUUID, err := uuid.Parse(chatID)
	if err != nil {
		sendJSONError(w, "Invalid chat ID format", http.StatusBadRequest)
		return
	}

	chat, err := h.ChatRepo.GetChatByID(chatUUID)
	if err != nil {
		sendJSONError(w, "Failed to get chat info", http.StatusInternalServerError)
		return
	}

	var messages []map[string]interface{}
	for _, msg := range resp.Messages {
		// используем nil для customKey, так как для исторических сообщений используем стандартный ключ
		decryptedMsg, err := h.ChatService.DecryptMessage(msg.EncryptedMessage, chat.Algorithm, chat.Mode, chat.Padding, nil)
		if err != nil {
			log.Println("Failed to decrypt message:", err)
			continue
		}

		dbMessages, err := h.ChatRepo.GetMessages(chatUUID)
		if err != nil {
			log.Println("Failed to fetch messages from database:", err)
		}

		messageData := map[string]interface{}{
			"sender_id":    msg.SenderId,
			"sender_name":  msg.SenderName,
			"created_at":   msg.CreatedAt,
			"message_type": "text",
			"text":         string(decryptedMsg),
		}

		for _, dbMsg := range dbMessages {
			if dbMsg.MessageType == "file" && dbMsg.SenderID.String() == msg.SenderId &&
				dbMsg.CreatedAt.UTC().Format(time.RFC3339) == msg.CreatedAt {
				messageData["message_type"] = "file"
				messageData["file_name"] = dbMsg.FileName
				base64Data := base64.StdEncoding.EncodeToString(decryptedMsg)
				messageData["message"] = base64Data
				messageData["is_base64"] = true
				delete(messageData, "text")
				break
			}
		}

		messages = append(messages, messageData)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messages,
	})
}

func (h *ChatHandlers) ExchangeKeyHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChatID    string `json:"chat_id"`
		ClientID  string `json:"client_id"`
		PublicKey string `json:"public_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(req.ClientID)
	if err != nil {
		sendJSONError(w, "Invalid client ID", http.StatusBadRequest)
		return
	}

	err = h.ChatRepo.SavePublicKey(r.Context(), userID, req.PublicKey)
	if err != nil {
		log.Println("Failed to save public key:", err)
		sendJSONError(w, "Failed to save public key", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (h *ChatHandlers) GetPeerKeysHandler(w http.ResponseWriter, r *http.Request) {
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		sendJSONError(w, "Chat ID is required", http.StatusBadRequest)
		return
	}

	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		sendJSONError(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	publicKeys, err := h.ChatRepo.GetPublicKeysByChatID(r.Context(), chatID)
	if err != nil {
		log.Println("Failed to get public keys:", err)
		sendJSONError(w, "Failed to get public keys", http.StatusInternalServerError)
		return
	}

	var response struct {
		Success    bool                `json:"success"`
		PublicKeys []map[string]string `json:"public_keys"`
	}
	response.Success = true
	response.PublicKeys = make([]map[string]string, 0, len(publicKeys))

	for userID, publicKey := range publicKeys {
		response.PublicKeys = append(response.PublicKeys, map[string]string{
			"client_id":  userID.String(),
			"public_key": publicKey,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
