package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sahata-worship-be/internal/handler"
	"sahata-worship-be/internal/repository"
	"sahata-worship-be/internal/usecase"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func loadEnv() {
	candidates := []string{".env", filepath.Join("..", "..", ".env")}
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := godotenv.Load(path); err != nil {
			log.Fatalf("gagal membaca %s: %v", path, err)
		}
		log.Printf("configuration loaded from %s", path)
		return
	}

	log.Println("file .env tidak ditemukan; menggunakan system environment/default value")
}

func main() {
	loadEnv()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local", env("DB_USER", "root"), env("DB_PASSWORD", ""), env("DB_HOST", "127.0.0.1"), env("DB_PORT", "3306"), env("DB_NAME", "sahata_worship"))
	db, e := sql.Open("mysql", dsn)
	if e != nil {
		log.Fatal(e)
	}
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	if e = db.Ping(); e != nil {
		log.Fatalf("database: %v", e)
	}
	h := handler.New(usecase.New(repository.NewMySQL(db), env("JWT_SECRET", "dev-secret-change-me")), env("CORS_ORIGIN", "http://localhost:5173"))
	// WriteTimeout must remain disabled for long-lived SSE signaling streams.
	srv := &http.Server{Addr: ":" + env("APP_PORT", "8080"), Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 0, IdleTimeout: 60 * time.Second}
	log.Printf("Sahata API listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
