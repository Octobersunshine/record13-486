//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	fmt.Println("=== Simple API Test ===")
	fmt.Println()

	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		fmt.Println("Health check failed:", err)
		return
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println("Health Check:", resp.Status, string(body))
	fmt.Println()

	resp, err = http.Post("http://localhost:8080/session/create", "application/json", nil)
	if err != nil {
		fmt.Println("Create session failed:", err)
		return
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println("Create Session:", resp.Status, string(body))

	var sessionResp map[string]interface{}
	json.Unmarshal(body, &sessionResp)
	sessionID := sessionResp["session_id"].(string)
	fmt.Println("Session ID:", sessionID)
	fmt.Println()

	testQuery := func(sql string) {
		fmt.Println("Query:", sql)
		reqBody, _ := json.Marshal(map[string]string{"sql": sql})
		req, _ := http.NewRequest("POST", "http://localhost:8080/query", bytes.NewBuffer(reqBody))
		req.Header.Set("X-Session-Id", sessionID)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("  Error:", err)
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Println("  Status:", resp.Status)
		fmt.Println("  Response:", string(body)[:min(200, len(body))])
		fmt.Println()
	}

	testQuery("SELECT * FROM users")
	testQuery("SELECT id, username FROM users WHERE age > 30")
	testQuery("INSERT INTO users (username) VALUES ('test')")
	testQuery("SELECT * FROM users WHERE age > 25 AND is_active = 1")
	testQuery("SELECT name, price FROM products WHERE price > 100")

	fmt.Println("=== Test Complete ===")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
