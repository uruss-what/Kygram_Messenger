package main

import (
	"Kygram/repository"
	"Kygram/services"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"Kygram/proto/protopb"

	"Kygram/config"
	//"Kygram/repository"
	server "Kygram/server"
	"Kygram/server/handlers"

	"google.golang.org/grpc"

	"github.com/gorilla/mux"
)

func main() {
	go server.StartGRPCServer()

	db := config.GetDB()
	defer db.Close()
	config.RunMigrations(db)

	userRepo := repository.NewUserRepository()
	chatRepo := repository.NewChatRepository(db)
	jwtKey := []byte("your-secret-key")
	authService := services.NewAuthService(userRepo, jwtKey)
	chatService := services.NewChatService(userRepo, chatRepo)

	grpcConn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer grpcConn.Close()

	chatClient := protopb.NewChatServiceClient(grpcConn)

	router := mux.NewRouter()

	staticPath, _ := filepath.Abs("web/static")
	router.PathPrefix("/Kygram/static/").Handler(http.StripPrefix("/Kygram/static/",
		http.FileServer(http.Dir(staticPath))))
	protoPath, _ := filepath.Abs("proto")
	router.PathPrefix("/proto/").Handler(http.StripPrefix("/proto/",
		http.FileServer(http.Dir(protoPath))))

	router.HandleFunc("/ws", handlers.NewChatHandlers(chatService, chatRepo, chatClient).WebSocketHandler)

	router.HandleFunc("/Kygram/auth", handlers.LoginPage)
	router.HandleFunc("/Kygram/dashboard", handlers.MainPage)
	router.HandleFunc("/Kygram/chat", handlers.ChatPage)

	authHandlers := handlers.NewAuthHandlers(authService)
	chatHandlers := handlers.NewChatHandlers(chatService, chatRepo, chatClient)

	router.HandleFunc("/register", authHandlers.RegisterHandler).Methods("POST")
	router.HandleFunc("/login", authHandlers.LoginHandler).Methods("POST")
	router.HandleFunc("/logout", authHandlers.LogoutHandler).Methods("POST")
	router.HandleFunc("/list-users", chatHandlers.ListUsersHandler).Methods("GET")
	router.HandleFunc("/list-user-chats", chatHandlers.ListUserChatsHandler).Methods("GET")
	router.HandleFunc("/create-chat", chatHandlers.CreateChatHandler).Methods("POST")
	router.HandleFunc("/send-message", chatHandlers.SendMessageHandler).Methods("POST")
	router.HandleFunc("/get-chat", chatHandlers.GetChatHandler).Methods("GET")
	router.HandleFunc("/messages", chatHandlers.GetMessagesHandler).Methods("GET")

	router.HandleFunc("/exchange-key", chatHandlers.ExchangeKeyHandler).Methods("POST")
	router.HandleFunc("/get-peer-keys", chatHandlers.GetPeerKeysHandler).Methods("GET")

	router.HandleFunc("/close-chat", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ChatID string `json:"chat_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		_, err := chatClient.CloseChat(r.Context(), &protopb.CloseChatRequest{
			ChatId: req.ChatID,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete chat: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}).Methods("POST")

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/Kygram/auth", http.StatusFound)
	})

	server := &http.Server{
		Addr:    ":2033",
		Handler: router,
	}

	log.Println("HTTP server running on :2033")
	log.Fatal(server.ListenAndServe())
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// package main

// import (
// 	"Kygram/algos"
// 	"bufio"
// 	"encoding/hex"
// 	"fmt"
// 	"os"
// )

// func main() {
// 	reader := bufio.NewReader(os.Stdin)
// 	fmt.Print("Введите текст для шифрования: ")
// 	inputText, _ := reader.ReadString('\n')
// 	inputText = inputText[:len(inputText)-1]

// 	key := []byte("thisis16bytekey!")
// 	iv := []byte("12345678abcdefgh")

// 	//////////////////////////////// rc5

// 	rc5, _ := algos.NewRC5()
// 	err := rc5.CipherKey(key)
// 	if err != nil {
// 		fmt.Println("Ошибка генерации ключей для rc5:", err)
// 		return
// 	}

// 	ctx := algos.NewEncryptionContext(
// 		key,
// 		algos.ECB,
// 		algos.Zeros,
// 		iv,
// 		rc5,
// 		rc5,
// 	)

// 	if ctx == nil {
// 		fmt.Println("Ошибка создания контекста шифрования rc5")
// 		return
// 	}

// 	encrypted, err := ctx.Encrypt([]byte(inputText))
// 	if err != nil {
// 		fmt.Printf("Ошибка шифрования: %v\n", err)
// 		return
// 	}
// 	fmt.Printf("Зашифрованный текст (в hex) RC5: %s\n", hex.EncodeToString(encrypted))

// 	decrypted, err := ctx.Decrypt(encrypted)
// 	if err != nil {
// 		fmt.Printf("Ошибка дешифрования: %v\n", err)
// 		return
// 	}
// 	fmt.Printf("Расшифрованный текст: %s\n\n\n", string(decrypted))

// 	///////////////////////// twofish

// 	twofish, _ := algos.NewTwofish()
// 	err2 := twofish.CipherKey(key)
// 	if err2 != nil {
// 		fmt.Println("Ошибка генерации ключей для twofish:", err2)
// 		return
// 	}

// 	ctx2 := algos.NewEncryptionContext(
// 		key,
// 		algos.RandomDelta,
// 		algos.ISO_10126,
// 		iv,
// 		twofish,
// 		twofish,
// 	)

// 	if ctx2 == nil {
// 		fmt.Println("Ошибка создания контекста шифрования twofish")
// 		return
// 	}

// 	encrypted2, err := ctx2.Encrypt([]byte(inputText))
// 	if err != nil {
// 		fmt.Printf("Ошибка шифрования: %v\n", err)
// 		return
// 	}
// 	fmt.Printf("Зашифрованный текст (в hex) Twofish: %s\n", hex.EncodeToString(encrypted2))

// 	decrypted2, err := ctx2.Decrypt(encrypted2)
// 	if err != nil {
// 		fmt.Printf("Ошибка дешифрования: %v\n", err)
// 		return
// 	}
// 	fmt.Printf("Расшифрованный текст: %s\n", string(decrypted2))
// }
