package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := openDatabase(filepath.Join(t.TempDir(), "newsroom.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := seed(db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestFreshDatabasePragmasAndIdempotentSeed(t *testing.T) {
	db := testDatabase(t)
	assertAgentCount(t, db, 3)

	var foreignKeys, busyTimeout int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || busyTimeout != 5000 {
		t.Fatalf("unexpected pragmas: foreign_keys=%d busy_timeout=%d", foreignKeys, busyTimeout)
	}
	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := seed(db); err != nil {
		t.Fatal(err)
	}
	assertAgentCount(t, db, 3)
}

func TestRestartPreservesExactlyThreeSeededAgents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "newsroom.db")
	db, err := openDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := seed(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = openDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := seed(db); err != nil {
		t.Fatal(err)
	}
	assertAgentCount(t, db, 3)
}

func TestReadAPIs(t *testing.T) {
	db := testDatabase(t)
	handler := newAPI(db, []string{"http://localhost:8081"}, testLogger(), nil)

	t.Run("health", func(t *testing.T) {
		response := request(t, handler, http.MethodGet, "/health", "")
		if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}\n" {
			t.Fatalf("unexpected health response: %d %s", response.Code, response.Body.String())
		}
	})
	t.Run("agents", func(t *testing.T) {
		response := request(t, handler, http.MethodGet, "/api/v1/agents", "")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.Code)
		}
		var body struct {
			Agents []struct {
				ID string `json:"id"`
			} `json:"agents"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Agents) != 3 {
			t.Fatalf("agent count = %d, want 3", len(body.Agents))
		}
	})
	t.Run("new conversation is empty", func(t *testing.T) {
		response := request(t, handler, http.MethodGet, "/api/v1/agents/bundesliga/messages", "")
		if response.Code != http.StatusOK || response.Body.String() != "{\"messages\":[]}\n" {
			t.Fatalf("unexpected messages response: %d %s", response.Code, response.Body.String())
		}
	})
	t.Run("unknown agent", func(t *testing.T) {
		response := request(t, handler, http.MethodGet, "/api/v1/agents/unknown/messages", "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", response.Code)
		}
		var body map[string]map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["error"]["code"] != "not_found" {
			t.Fatalf("error code = %q, want not_found", body["error"]["code"])
		}
	})
	t.Run("configured CORS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "http://localhost:8081")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8081" {
			t.Fatalf("missing configured CORS header")
		}
	})
}

func assertAgentCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("agent count = %d, want %d", got, want)
	}
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
