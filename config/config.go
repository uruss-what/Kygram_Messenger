package config

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"

	"github.com/streadway/amqp"

	_ "github.com/lib/pq"
)

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла:", err)
	}
}

func GetDB() *sql.DB {
	LoadEnv()
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	//log.Println("Подключение к БД:", dsn)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}

	return db
}

func RunMigrations(db *sql.DB) {
	filePath := "docker/postgres-init.sql"

	sqlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Ошибка чтения файла %s: %v", filePath, err)
	}

	_, err = db.Exec(string(sqlFile))
	if err != nil {
		log.Fatalf("Ошибка выполнения миграций: %v", err)
	}

	log.Println("Миграции выполнены успешно")
}

func GetRabbitMQ() (*amqp.Connection, error) {
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/", "user", "password", os.Getenv("RABBITMQ_HOST"), 5672))
	if err != nil {
		return nil, err
	}
	return conn, nil
}

var ctx = context.Background()

func GetRedisClient() *redis.Client {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: "",
		DB:       0,
	})

	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Ошибка подключения к Redis: %v", err)
	}

	return client
}

func SetCache(key string, value string, expiration time.Duration) {
	client := GetRedisClient()
	defer client.Close()

	err := client.Set(ctx, key, value, expiration).Err()
	if err != nil {
		log.Printf("Ошибка записи в Redis: %v", err)
	}
}

func GetCache(key string) (string, error) {
	client := GetRedisClient()
	defer client.Close()

	val, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", err
	}

	return val, nil
}
