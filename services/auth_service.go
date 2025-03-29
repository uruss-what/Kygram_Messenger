package services

import (
	"context"
	"errors"
	"time"

	"Kygram/proto/protopb"
	"Kygram/repository"

	"github.com/golang-jwt/jwt"
	//"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	protopb.UnimplementedUserServiceServer
	userRepo *repository.UserRepository
	jwtKey   []byte
}

func NewAuthService(repo *repository.UserRepository, jwtKey []byte) *AuthService {
	return &AuthService{userRepo: repo, jwtKey: jwtKey}
}

func (s *AuthService) RegisterUser(ctx context.Context, req *protopb.RegisterRequest) (*protopb.RegisterResponse, error) {
	_, _, _, err := s.userRepo.GetUser(req.Username)
	if err == nil {
		return &protopb.RegisterResponse{Success: false, Message: "User already exists"}, errors.New("user already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return &protopb.RegisterResponse{Success: false, Message: "Error hashing password"}, err
	}

	err = s.userRepo.CreateUser(req.Username, string(hashedPassword))
	if err != nil {
		return &protopb.RegisterResponse{Success: false, Message: "Failed to create user"}, err
	}

	return &protopb.RegisterResponse{Success: true, Message: "User registered successfully"}, nil
}

func (s *AuthService) Login(ctx context.Context, req *protopb.LoginRequest) (*protopb.LoginResponse, error) {
	userID, username, passwordHash, err := s.userRepo.GetUser(req.Username)
	if err != nil {
		return &protopb.LoginResponse{Success: false, Message: "User not found"}, errors.New("user not found")
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		return &protopb.LoginResponse{Success: false, Message: "Invalid credentials"}, errors.New("invalid credentials")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"user_id":  userID,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(s.jwtKey)
	if err != nil {
		return &protopb.LoginResponse{Success: false, Message: "Failed to generate token"}, err
	}

	return &protopb.LoginResponse{
		Success: true,
		Token:   tokenString,
		UserId:  userID.String(),
		Message: "Login successful",
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, req *protopb.LogoutRequest) (*protopb.LogoutResponse, error) {
	// добавить инвалидацию токена через редис
	return &protopb.LogoutResponse{
		Success: true,
		Message: "Logout successful",
	}, nil
}
