package handlers

import (
	"Kygram/proto/protopb"
	"Kygram/services"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"text/template"
)

func SignUpPage(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.ParseFiles("web/templates/sign_up.html")
	tmpl.Execute(w, nil)
}

func LoginPage(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.ParseFiles("web/templates/sign_in.html")
	tmpl.Execute(w, nil)
}

func MainPage(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.ParseFiles("web/templates/main.html")
	tmpl.Execute(w, nil)
}

func ChatPage(w http.ResponseWriter, r *http.Request) {
	tmpl, _ := template.ParseFiles("web/templates/chat.html")
	tmpl.Execute(w, nil)
}

type AuthHandlers struct {
	AuthService *services.AuthService
}

func NewAuthHandlers(authService *services.AuthService) *AuthHandlers {
	return &AuthHandlers{AuthService: authService}
}

func (h *AuthHandlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req protopb.RegisterRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.AuthService.RegisterUser(context.Background(), &req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AuthHandlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req protopb.LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.AuthService.Login(context.Background(), &req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AuthHandlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Authorization header is required", http.StatusUnauthorized)
		return
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)

	req := &protopb.LogoutRequest{
		Token: token,
	}

	resp, err := h.AuthService.Logout(context.Background(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
