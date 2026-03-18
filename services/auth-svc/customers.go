package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Customer struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *Customer) validateNew() error {
	if !strings.Contains(c.Email, "@") || len(c.Email) < 5 {
		return errors.New("invalid email")
	}
	if strings.TrimSpace(c.FullName) == "" {
		return errors.New("full_name required")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func parseIntDefault(s string, def int) int {
	i, err := strconv.Atoi(s)
	if s == "" || err != nil {
		return def
	}
	return i
}

func createCustomerHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	c := &Customer{Email: strings.TrimSpace(in.Email), FullName: strings.TrimSpace(in.FullName)}
	if err := c.validateNew(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := dbConn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close(context.Background())

	id := uuid.New()
	row := conn.QueryRow(context.Background(),
		`INSERT INTO customers (id, email, full_name)
         VALUES ($1, $2, $3)
         RETURNING id, email, full_name, created_at, updated_at`,
		id, c.Email, c.FullName)
	if err := row.Scan(&c.ID, &c.Email, &c.FullName, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			http.Error(w, "email already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func getCustomerHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	conn, err := dbConn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close(context.Background())

	var c Customer
	row := conn.QueryRow(context.Background(),
		`SELECT id, email, full_name, created_at, updated_at
           FROM customers WHERE id=$1`, id)
	if err := row.Scan(&c.ID, &c.Email, &c.FullName, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, &c)
}

func listCustomersHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := parseIntDefault(r.URL.Query().Get("limit"), 20)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	conn, err := dbConn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close(context.Background())

	var rows pgx.Rows
	if q == "" {
		rows, err = conn.Query(context.Background(),
			`SELECT id, email, full_name, created_at, updated_at
               FROM customers
              ORDER BY created_at DESC
              LIMIT $1 OFFSET $2`, limit, offset)
	} else {
		p := "%" + strings.ToLower(q) + "%"
		rows, err = conn.Query(context.Background(),
			`SELECT id, email, full_name, created_at, updated_at
               FROM customers
              WHERE lower(email) LIKE $1 OR lower(full_name) LIKE $1
              ORDER BY created_at DESC
              LIMIT $2 OFFSET $3`, p, limit, offset)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var out []Customer
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.Email, &c.FullName, &c.CreatedAt, &c.UpdatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  out,
		"limit":  limit,
		"offset": offset,
		"query":  q,
	})
}

func updateCustomerHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var in struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	c := &Customer{Email: strings.TrimSpace(in.Email), FullName: strings.TrimSpace(in.FullName)}
	if err := c.validateNew(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := dbConn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close(context.Background())

	row := conn.QueryRow(context.Background(),
		`UPDATE customers
            SET email=$1,
                full_name=$2,
                updated_at=now()
          WHERE id=$3
      RETURNING id, email, full_name, created_at, updated_at`,
		c.Email, c.FullName, id)
	if err := row.Scan(&c.ID, &c.Email, &c.FullName, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			http.Error(w, "email already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func deleteCustomerHandler(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	conn, err := dbConn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close(context.Background())

	ct, err := conn.Exec(context.Background(), `DELETE FROM customers WHERE id=$1`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ct.RowsAffected() == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
