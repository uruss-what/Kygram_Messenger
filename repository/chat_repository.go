package repository

import (
	"context"
	"database/sql"
	"fmt"

	"Kygram/models"

	"github.com/google/uuid"
)

type ChatRepository struct {
	db *sql.DB
}
type Chat struct {
	ID   uuid.UUID `json:"chat_id"`
	Name string    `json:"name"`
}

func NewChatRepository(db *sql.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) CreateChat(ctx context.Context, chatID uuid.UUID, name, algorithm, mode, padding, prime string, participants []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO chats (chat_id, name, algorithm, mode, padding, prime) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err = tx.ExecContext(ctx, query, chatID, name, algorithm, mode, padding, prime)
	if err != nil {
		return fmt.Errorf("failed to insert chat: %w", err)
	}

	query = `INSERT INTO chat_participants (chat_id, user_id) VALUES ($1, $2)`
	for _, participant := range participants {
		_, err = tx.ExecContext(ctx, query, chatID, participant)
		if err != nil {
			return fmt.Errorf("failed to insert participant: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *ChatRepository) DeleteChat(ctx context.Context, chatID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `DELETE FROM chat_participants WHERE chat_id = $1`
	_, err = tx.ExecContext(ctx, query, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete chat participants: %w", err)
	}

	query = `DELETE FROM messages WHERE chat_id = $1`
	_, err = tx.ExecContext(ctx, query, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete chat messages: %w", err)
	}

	query = `DELETE FROM chats WHERE chat_id = $1`
	_, err = tx.ExecContext(ctx, query, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete chat: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *ChatRepository) GetUserChats(userID uuid.UUID) ([]models.Chat, error) {
	var chats []models.Chat
	query := `
        SELECT c.chat_id, c.name
        FROM chats c
        JOIN chat_participants cp ON c.chat_id = cp.chat_id
        WHERE cp.user_id = $1
    `
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user chats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var chat models.Chat
		if err := rows.Scan(&chat.ChatID, &chat.Name); err != nil {
			return nil, fmt.Errorf("failed to scan chat: %w", err)
		}
		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating rows: %w", err)
	}

	return chats, nil
}
func (r *ChatRepository) GetChatByID(chatID uuid.UUID) (*models.Chat, error) {
	query := `
        SELECT chat_id, name, algorithm, mode, padding, prime, created_at
        FROM chats
        WHERE chat_id = $1
    `

	var chat models.Chat
	r.db.QueryRowContext(context.Background(), query, chatID).Scan(
		&chat.ChatID,
		&chat.Name,
		&chat.Algorithm,
		&chat.Mode,
		&chat.Padding,
		&chat.Prime,
		&chat.CreatedAt,
	)

	return &chat, nil
}

func (r *ChatRepository) GetChatParticipants(chatID uuid.UUID) ([]models.Participant, error) {
	var participants []models.Participant
	query := `
        SELECT u.user_id::text, u.username 
        FROM users u
        JOIN chat_participants cp ON u.user_id = cp.user_id
        WHERE cp.chat_id = $1
    `
	rows, err := r.db.Query(query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query participants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p models.Participant
		if err := rows.Scan(&p.UserID, &p.Username); err != nil {
			return nil, fmt.Errorf("failed to scan participant: %w", err)
		}
		participants = append(participants, p)
	}

	return participants, nil
}

func (r *ChatRepository) SaveMessage(ctx context.Context, msg models.Message) error {
	query := `
        INSERT INTO messages (
            message_id,
            chat_id,
            sender_id,
            encrypted_message,
            created_at,
            message_type,
            file_name,
            chunk_index,
            total_chunks
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `
	_, err := r.db.ExecContext(
		ctx,
		query,
		msg.MessageID,
		msg.ChatID,
		msg.SenderID,
		msg.EncryptedMessage,
		msg.CreatedAt,
		msg.MessageType,
		msg.FileName,
		msg.ChunkIndex,
		msg.TotalChunks,
	)
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	return nil
}

func (r *ChatRepository) GetMessages(chatID uuid.UUID) ([]models.Message, error) {
	query := `
        SELECT message_id, chat_id, sender_id, encrypted_message, created_at, message_type, file_name, chunk_index, total_chunks
        FROM messages
        WHERE chat_id = $1
        ORDER BY created_at ASC
    `
	rows, err := r.db.Query(query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(
			&msg.MessageID,
			&msg.ChatID,
			&msg.SenderID,
			&msg.EncryptedMessage,
			&msg.CreatedAt,
			&msg.MessageType,
			&msg.FileName,
			&msg.ChunkIndex,
			&msg.TotalChunks,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iterating rows: %w", err)
	}

	return messages, nil
}

func (r *ChatRepository) ChatExists(ctx context.Context, chatID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM chats WHERE chat_id = $1)`
	err := r.db.QueryRowContext(ctx, query, chatID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check chat existence: %w", err)
	}
	return exists, nil
}

func (r *UserRepository) GetUsernameByID(ctx context.Context, userID uuid.UUID) (string, error) {
	var username string
	query := `SELECT username FROM users WHERE user_id = $1`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(&username)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("user not found")
		}
		return "", fmt.Errorf("failed to get username: %w", err)
	}

	return username, nil
}

func (r *ChatRepository) SavePublicKey(ctx context.Context, userID uuid.UUID, publicKey string) error {
	query := `
		INSERT INTO session_keys (user_id, public_key)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET
		public_key = EXCLUDED.public_key
	`
	_, err := r.db.ExecContext(ctx, query, userID, publicKey)
	return err
}

func (r *ChatRepository) GetPublicKeyByUserID(ctx context.Context, userID uuid.UUID) (string, error) {
	var publicKey string
	query := `SELECT public_key FROM session_keys WHERE user_id = $1`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(&publicKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("public key not found for user %s", userID)
		}
		return "", fmt.Errorf("failed to get public key: %w", err)
	}

	return publicKey, nil
}

func (r *ChatRepository) GetPublicKeysByChatID(ctx context.Context, chatID uuid.UUID) (map[uuid.UUID]string, error) {
	query := `		SELECT u.user_id, sk.public_key
		FROM chat_participants cp
		JOIN users u ON cp.user_id = u.user_id
		LEFT JOIN session_keys sk ON u.user_id = sk.user_id
		WHERE cp.chat_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	publicKeys := make(map[uuid.UUID]string)
	for rows.Next() {
		var userID uuid.UUID
		var publicKey sql.NullString
		if err := rows.Scan(&userID, &publicKey); err != nil {
			return nil, err
		}
		if publicKey.Valid {
			publicKeys[userID] = publicKey.String
		}
	}

	return publicKeys, nil
}

func (r *ChatRepository) DeletePublicKey(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM session_keys WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete public key: %w", err)
	}
	return nil
}
