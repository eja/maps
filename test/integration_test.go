// Copyright (C) by Ubaldo Porcheddu <ubaldo@eja.it>

package test

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eja/maps"
	_ "modernc.org/sqlite"
)

// setupTestDB creates a valid MBTiles file for testing
func setupTestDB(t *testing.T, dir, filename string) string {
	path := filepath.Join(dir, filename)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("Failed to create test db: %v", err)
	}
	defer db.Close()

	queries := []string{
		`CREATE TABLE metadata (name text, value text);`,
		`CREATE TABLE tiles (zoom_level integer, tile_column integer, tile_row integer, tile_data blob);`,
		`INSERT INTO metadata (name, value) VALUES ('name', 'Test Map');`,
		`INSERT INTO metadata (name, value) VALUES ('format', 'png');`,
		// Insert Z=1, X=0, Y=0 -> TMS Row 1
		`INSERT INTO tiles (zoom_level, tile_column, tile_row, tile_data) VALUES (1, 0, 1, x'89504E47');`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("Failed to execute query '%s': %v", q, err)
		}
	}
	return path
}

func TestMapsServer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "maps_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	setupTestDB(t, tmpDir, "world.mbtiles")

	webPath := "/maps/"
	handler := maps.New(webPath, tmpDir, "")
	server := httptest.NewServer(handler)
	defer server.Close()

	client := server.Client()

	t.Run("Metadata JSON", func(t *testing.T) {
		resp, err := client.Get(server.URL + "/maps/map/world.mbtiles/metadata.json")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		expected := `Test Map`
		if !strings.Contains(string(body), expected) {
			t.Errorf("Expected metadata to contain '%s', got: %s", expected, body)
		}
	})

	t.Run("Serve Tile (Coordinate Conversion Check)", func(t *testing.T) {
		url := server.URL + "/maps/map/world.mbtiles/1/0/0"
		resp, err := client.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK for valid tile, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if len(body) < 4 || fmt.Sprintf("%X", body[0:4]) != "89504E47" {
			t.Errorf("Tile data mismatch. Expected header 89504E47, got %X", body)
		}
	})

	t.Run("Missing Tile", func(t *testing.T) {
		url := server.URL + "/maps/map/world.mbtiles/1/0/99"
		resp, err := client.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 for missing tile, got %d", resp.StatusCode)
		}
	})

	t.Run("Path Traversal Attack", func(t *testing.T) {
		url := server.URL + "/maps/map/../../etc/passwd"
		resp, err := client.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Error("Security fail: Request for ../ succeeded")
		}
	})
}

func TestAuthentication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "maps_auth_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	authClear := "user:pass"
	authEncoded := base64.StdEncoding.EncodeToString([]byte(authClear))

	handler := maps.New("/maps/", tmpDir, authEncoded)
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("No Header", func(t *testing.T) {
		resp, err := server.Client().Get(server.URL + "/maps/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected 401 Unauthorized, got %d", resp.StatusCode)
		}
	})

	t.Run("Correct Header", func(t *testing.T) {
		req, _ := http.NewRequest("GET", server.URL+"/maps/", nil)
		authHeaderVal := base64.StdEncoding.EncodeToString([]byte(authClear))
		req.Header.Add("Authorization", "Basic "+authHeaderVal)

		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			t.Error("Authentication failed with correct credentials")
		}
	})
}
