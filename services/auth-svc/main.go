package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	v "github.com/clarence-gray/go-zero-trust-shop/auth-svc/vault"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func dbConn() (*pgx.Conn, error) {
	addr := v.EnvOrDie("VAULT_ADDR")
	roleID := v.EnvOrDie("VAULT_ROLE_ID")
	secretID := v.EnvOrDie("VAULT_SECRET_ID")

	// 1) Login to Vault with AppRole
	token, err := v.LoginWithAppRole(addr, roleID, secretID)
	if err != nil {
		return nil, fmt.Errorf("approle login: %w", err)
	}

	// 2) Fetch dynamic DB credentials
	user, pass, err := v.GetDynamicDBCreds(addr, token, "app-role")
	if err != nil {
		return nil, fmt.Errorf("db creds: %w", err)
	}

	// 3) Build pgx config (no DSN parsing), force IPv4 host
	cfg, err := pgx.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.Host = "127.0.0.1"
	cfg.Port = 55432 // Option B: host 55432 -> container 5432
	cfg.Database = "shop"
	cfg.User = user
	cfg.Password = pass
	cfg.TLSConfig = nil

	// 4) Short, bounded retry to absorb "brand-new role not ready" races
	const maxAttempts = 5
	var conn *pgx.Conn
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err = pgx.ConnectConfig(context.Background(), cfg)
		if err == nil {
			return conn, nil
		}

		// Only retry auth-ish failures; bail on everything else
		lerr := strings.ToLower(err.Error())
		if !strings.Contains(lerr, "sasl") &&
			!strings.Contains(lerr, "password authentication failed") &&
			!strings.Contains(lerr, "no pg_hba.conf entry") { // non-auth errors: return fast
			return nil, fmt.Errorf("connect: %w", err)
		}

		// Exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms
		backoff := time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond
		log.Printf("DEBUG connect retry %d/%d after %v (err=%v)", attempt, maxAttempts, backoff, err)
		time.Sleep(backoff)
	}

	return nil, fmt.Errorf("connect after retries: %w", err)
}

func main() {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Get("/db/ping", func(w http.ResponseWriter, r *http.Request) {
		conn, err := dbConn()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close(context.Background())

		var one int
		if err := conn.QueryRow(context.Background(), "select 1").Scan(&one); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(fmt.Sprintf("db: %d", one)))
	})

	// Allow overriding port via PORT; default 8081
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Println("auth-svc listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
