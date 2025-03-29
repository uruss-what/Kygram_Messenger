package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"Kygram/config"

	_ "github.com/lib/pq"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository() *UserRepository {
	db := config.GetDB()
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateUser(username, password string) error {
	_, err := r.db.Exec("INSERT INTO users (username, password_hash) VALUES ($1, $2)", username, password)
	return err
}

func (r *UserRepository) GetUser(username string) (uuid.UUID, string, string, error) {
	var userID uuid.UUID
	var passwordHash string
	query := `SELECT user_id, password_hash FROM users WHERE username = $1`
	err := r.db.QueryRow(query, username).Scan(&userID, &passwordHash)
	if err != nil {
		return uuid.Nil, "", "", err
	}
	return userID, username, passwordHash, nil
}

func (r *UserRepository) ListUsers() ([]string, error) {
	rows, err := r.db.Query("SELECT username FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		users = append(users, username)
	}
	return users, nil
}
func (r *UserRepository) GetUserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	var userID uuid.UUID
	query := `SELECT user_id FROM users WHERE username = $1`
	err := r.db.QueryRowContext(ctx, query, username).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, fmt.Errorf("user with username %s not found", username)
		}
		return uuid.Nil, fmt.Errorf("failed to get user ID by username: %w", err)
	}
	return userID, nil
}
