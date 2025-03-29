package models

import (
	"time"

	"github.com/google/uuid"
)

type Chat struct {
	ChatID    uuid.UUID `json:"chat_id"`
	Name      string    `json:"name"`
	Algorithm string    `json:"algorithm"`
	Mode      string    `json:"mode"`
	Padding   string    `json:"padding"`
	Prime     string    `json:"prime"`
	CreatedAt time.Time `json:"created_at"`
}

type Participant struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	PublicKey string    `json:"public_key"`
}
type Message struct {
	MessageID        uuid.UUID `json:"message_id"`
	ChatID           uuid.UUID `json:"chat_id"`
	SenderID         uuid.UUID `json:"sender_id"`
	SenderName       string    `json:"sender_name"`
	EncryptedMessage []byte    `json:"encrypted_message"`
	CreatedAt        time.Time `json:"created_at"`
	MessageType      string    `json:"message_type"`
	FileName         string    `json:"file_name"`
	ChunkIndex       int       `json:"chunk_index"`
	TotalChunks      int       `json:"total_chunks"`
}
