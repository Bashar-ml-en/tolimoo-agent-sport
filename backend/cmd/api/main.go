// Command api runs the single-process newsroom HTTP API.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

type config struct {
	addr, dbPath string
	cors         []string
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config{env("HTTP_ADDR", ":8080"), env("DATABASE_PATH", "./data/newsroom.db"), strings.Split(env("CORS_ALLOWED_ORIGINS", "http://localhost:8081,http://localhost:19006"), ",")}
	if err := os.MkdirAll(filepath.Dir(cfg.dbPath), 0o755); err != nil {
		log.Error("create database directory", "error", err)
		os.Exit(1)
	}
	db, err := sql.Open("sqlite", cfg.dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		log.Error("connect database", "error", err)
		os.Exit(1)
	}
	if err := migrate(db); err != nil {
		log.Error("migrate database", "error", err)
		os.Exit(1)
	}
	if err := seed(db); err != nil {
		log.Error("seed database", "error", err)
		os.Exit(1)
	}

	h := newAPI(db, cfg.cors, log)
	server := &http.Server{Addr: cfg.addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() { log.Info("API listening", "addr", cfg.addr) }()
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}

func jsonError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func notImplemented(w http.ResponseWriter) {
	jsonError(w, http.StatusNotImplemented, "not_implemented", "This operation is not implemented yet.")
}
func boundedJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(dst)
}

// migrate is intentionally small and repeatable: each numbered SQL file is recorded once.
func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(schema)
	return err
}

const schema = `CREATE TABLE IF NOT EXISTS agents (id TEXT PRIMARY KEY, assignment TEXT NOT NULL, language TEXT NOT NULL, platforms TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1, research_interval_seconds INTEGER NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS runs (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES agents(id), status TEXT NOT NULL, started_at TEXT NOT NULL, ended_at TEXT, error TEXT);
CREATE TABLE IF NOT EXISTS stories (id TEXT PRIMARY KEY, deduplication_key TEXT NOT NULL UNIQUE, title TEXT NOT NULL, summary TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sources (id TEXT PRIMARY KEY, story_id TEXT NOT NULL REFERENCES stories(id), url TEXT NOT NULL, title TEXT NOT NULL, published_at TEXT, retrieved_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS drafts (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES agents(id), story_id TEXT NOT NULL REFERENCES stories(id), run_id TEXT NOT NULL REFERENCES runs(id), headline TEXT NOT NULL, claim_status TEXT NOT NULL, facebook_text TEXT NOT NULL, x_text TEXT NOT NULL, review_status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS messages (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES agents(id), role TEXT NOT NULL, message_type TEXT NOT NULL, text TEXT NOT NULL, draft_id TEXT REFERENCES drafts(id), run_id TEXT REFERENCES runs(id), created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_messages_agent_created ON messages(agent_id, created_at); CREATE INDEX IF NOT EXISTS idx_runs_agent_status ON runs(agent_id, status); CREATE INDEX IF NOT EXISTS idx_drafts_agent_review ON drafts(agent_id, review_status); CREATE INDEX IF NOT EXISTS idx_sources_story ON sources(story_id);`

func seed(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	agents := []struct{ id, assignment string }{{"premier_league", "Premier League news"}, {"bundesliga", "Bundesliga transfers"}, {"coach_statements", "Coach statements"}}
	for _, a := range agents {
		_, err := db.Exec(`INSERT INTO agents (id, assignment, language, platforms, enabled, research_interval_seconds, created_at, updated_at) VALUES (?, ?, 'fr', '["facebook","x"]', 1, 1800, ?, ?) ON CONFLICT(id) DO NOTHING`, a.id, a.assignment, now, now)
		if err != nil {
			return err
		}
	}
	return nil
}

type api struct {
	db   *sql.DB
	cors map[string]bool
	log  *slog.Logger
}

func newAPI(db *sql.DB, origins []string, log *slog.Logger) http.Handler {
	cors := map[string]bool{}
	for _, origin := range origins {
		cors[strings.TrimSpace(origin)] = true
	}
	a := &api{db: db, cors: cors, log: log}
	return a.middleware(http.HandlerFunc(a.route))
}
func (a *api) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if a.cors[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				a.log.Error("panic recovered", "error", recovered)
				jsonError(w, 500, "internal_error", "Internal server error.")
			}
		}()
		started := time.Now()
		next.ServeHTTP(w, r)
		a.log.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}
func (a *api) route(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if r.Method == "GET" && path == "health" {
		jsonResponse(w, 200, map[string]string{"status": "ok"})
		return
	}
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "agents" {
		a.agents(w, r, parts[3:])
		return
	}
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "drafts" && r.Method == "PATCH" {
		notImplemented(w)
		return
	}
	jsonError(w, 404, "not_found", "Route not found.")
}
func (a *api) agents(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 && r.Method == "GET" {
		rows, err := a.db.Query(`SELECT id, assignment, language, platforms, enabled, research_interval_seconds, created_at, updated_at FROM agents ORDER BY id`)
		if err != nil {
			jsonError(w, 500, "database_error", "Could not list agents.")
			return
		}
		defer rows.Close()
		result := []map[string]any{}
		for rows.Next() {
			var id, assignment, language, platforms, created, updated string
			var enabled, interval int
			if err := rows.Scan(&id, &assignment, &language, &platforms, &enabled, &interval, &created, &updated); err != nil {
				jsonError(w, 500, "database_error", "Could not read agents.")
				return
			}
			var platformList []string
			_ = json.Unmarshal([]byte(platforms), &platformList)
			var pending, running int
			_ = a.db.QueryRow(`SELECT COUNT(*) FROM drafts WHERE agent_id=? AND review_status='pending'`, id).Scan(&pending)
			_ = a.db.QueryRow(`SELECT COUNT(*) FROM runs WHERE agent_id=? AND status IN ('queued','running')`, id).Scan(&running)
			result = append(result, map[string]any{"id": id, "assignment": assignment, "language": language, "platforms": platformList, "enabled": enabled == 1, "researchIntervalSeconds": interval, "pendingDraftCount": pending, "isRunning": running > 0, "createdAt": created, "updatedAt": updated})
		}
		jsonResponse(w, 200, map[string]any{"agents": result})
		return
	}
	if len(rest) == 2 && rest[1] == "messages" && r.Method == "GET" {
		a.messages(w, rest[0])
		return
	}
	if len(rest) == 2 && rest[1] == "runs" && r.Method == "POST" {
		notImplemented(w)
		return
	}
	jsonError(w, 404, "not_found", "Route not found.")
}
func (a *api) messages(w http.ResponseWriter, agentID string) {
	rows, err := a.db.Query(`SELECT id, role, message_type, text, draft_id, run_id, created_at FROM messages WHERE agent_id=? ORDER BY created_at ASC, id ASC`, agentID)
	if err != nil {
		jsonError(w, 500, "database_error", "Could not read messages.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, role, typ, text, created string
		var draft, run sql.NullString
		if err := rows.Scan(&id, &role, &typ, &text, &draft, &run, &created); err != nil {
			jsonError(w, 500, "database_error", "Could not read messages.")
			return
		}
		item := map[string]any{"id": id, "agentId": agentID, "role": role, "messageType": typ, "text": text, "draftId": nil, "runId": nil, "createdAt": created}
		if draft.Valid {
			item["draftId"] = draft.String
		}
		if run.Valid {
			item["runId"] = run.String
		}
		items = append(items, item)
	}
	jsonResponse(w, 200, map[string]any{"messages": items})
}
