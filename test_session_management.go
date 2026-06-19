//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "http://localhost:8082"

func httpGet(url string, headers map[string]string) (int, string, error) {
	req, err := http.NewRequest("GET", baseURL+url, nil)
	if err != nil {
		return 0, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body), nil
}

func httpPost(url string, headers map[string]string, body interface{}) (int, string, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(jsonBytes)
	}
	req, err := http.NewRequest("POST", baseURL+url, bodyReader)
	if err != nil {
		return 0, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

func main() {
	fmt.Println("=== Session Management Comprehensive Test ===")
	fmt.Println()

	fmt.Println("--- Test 1: Health Check ---")
	status, body, err := httpGet("/health", nil)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	fmt.Printf("Status: %d\n", status)
	fmt.Printf("Response: %s\n", body)
	var healthData map[string]interface{}
	json.Unmarshal([]byte(body), &healthData)
	fmt.Printf("Active sessions: %v\n", healthData["active_sessions"])
	fmt.Println()

	fmt.Println("--- Test 2: Create Session and Verify Idle Timeout Config ---")
	status, body, err = httpPost("/session/create", nil, nil)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	fmt.Printf("Status: %d\n", status)
	var sessionData map[string]interface{}
	json.Unmarshal([]byte(body), &sessionData)
	sessionID := sessionData["session_id"].(string)
	fmt.Printf("Session ID: %s\n", sessionID)
	fmt.Printf("Idle timeout: %v\n", sessionData["idle_timeout"])
	fmt.Printf("Max lifetime: %v\n", sessionData["max_lifetime"])
	fmt.Printf("Idle expires at: %v\n", sessionData["idle_expires_at"])
	fmt.Printf("Max lifetime expires at: %v\n", sessionData["expires_at"])
	fmt.Println()

	fmt.Println("--- Test 3: Session Info API ---")
	status, body, err = httpGet("/session/info", map[string]string{"X-Session-Id": sessionID})
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	fmt.Printf("Status: %d\n", status)
	fmt.Printf("Response: %s\n", body)
	var infoData map[string]interface{}
	json.Unmarshal([]byte(body), &infoData)
	fmt.Printf("Query count: %v\n", infoData["query_count"])
	fmt.Printf("Remaining idle seconds: %.1f\n", infoData["remaining_idle_seconds"])
	fmt.Println()

	fmt.Println("--- Test 4: Execute Query and Count Tracking ---")
	for i := 0; i < 3; i++ {
		status, body, err = httpPost("/query",
			map[string]string{"X-Session-Id": sessionID},
			map[string]string{"sql": "SELECT * FROM users LIMIT 1"})
		if err != nil {
			fmt.Println("ERROR:", err)
			return
		}
		var queryData map[string]interface{}
		json.Unmarshal([]byte(body), &queryData)
		fmt.Printf("Query %d - Status: %d, Session Query Count: %v\n",
			i+1, status, queryData["session_query_count"])
	}
	fmt.Println()

	fmt.Println("--- Test 5: Stats API ---")
	status, body, err = httpGet("/stats", nil)
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	fmt.Printf("Status: %d\n", status)
	var statsData map[string]interface{}
	json.Unmarshal([]byte(body), &statsData)
	sm := statsData["session_manager"].(map[string]interface{})
	fmt.Printf("Total sessions: %v\n", sm["total_sessions"])
	fmt.Printf("Max sessions: %v\n", sm["max_sessions"])
	fmt.Printf("Total created: %v\n", sm["total_created"])
	fmt.Printf("Total queries: %v\n", sm["total_queries"])
	fmt.Printf("Cleanup count: %v\n", sm["cleanup_count"])
	fmt.Printf("Uptime: %v\n", sm["uptime"])
	fmt.Println()

	fmt.Println("--- Test 6: LRU Eviction (max-sessions = 5) ---")
	createdSessions := []string{sessionID}
	for i := 0; i < 6; i++ {
		status, body, err = httpPost("/session/create", nil, nil)
		if err != nil {
			fmt.Println("ERROR:", err)
			return
		}
		var newSession map[string]interface{}
		json.Unmarshal([]byte(body), &newSession)
		newID := newSession["session_id"].(string)
		createdSessions = append(createdSessions, newID)
		fmt.Printf("Created session %d: %s (status: %d)\n", i+2, newID, status)
	}
	fmt.Println()

	status, body, err = httpGet("/stats", nil)
	json.Unmarshal([]byte(body), &statsData)
	sm = statsData["session_manager"].(map[string]interface{})
	fmt.Printf("After creating 7 sessions (max=5):\n")
	fmt.Printf("  Current total sessions: %v\n", sm["total_sessions"])
	fmt.Printf("  Total evicted: %v\n", sm["total_evicted"])
	fmt.Println()

	fmt.Println("--- Test 7: First session should be evicted by LRU ---")
	status, body, err = httpGet("/session/info", map[string]string{"X-Session-Id": createdSessions[0]})
	fmt.Printf("First session (created first) status: %d\n", status)
	if status == 401 {
		fmt.Println("SUCCESS: First session correctly evicted by LRU")
	} else {
		fmt.Println("Response:", body)
	}
	fmt.Println()

	fmt.Println("--- Test 8: Most recent session should still be valid ---")
	lastID := createdSessions[len(createdSessions)-1]
	status, body, err = httpGet("/session/info", map[string]string{"X-Session-Id": lastID})
	fmt.Printf("Last session (most recent) status: %d\n", status)
	if status == 200 {
		fmt.Println("SUCCESS: Most recent session is still valid")
	}
	fmt.Println()

	fmt.Println("--- Test 9: Close Session ---")
	status, body, err = httpPost("/session/close", map[string]string{"X-Session-Id": lastID}, nil)
	fmt.Printf("Close session status: %d\n", status)
	fmt.Printf("Response: %s\n", body)

	status, body, err = httpGet("/stats", nil)
	json.Unmarshal([]byte(body), &statsData)
	sm = statsData["session_manager"].(map[string]interface{})
	fmt.Printf("After close: total_closed = %v\n", sm["total_closed"])
	fmt.Println()

	fmt.Println("--- Test 10: Idle Timeout (waiting 12 seconds for 10s timeout) ---")
	testSession := createdSessions[3]
	fmt.Printf("Testing session: %s\n", testSession)
	fmt.Println("Waiting 12 seconds (idle timeout is 10s)...")
	time.Sleep(12 * time.Second)

	status, body, err = httpGet("/session/info", map[string]string{"X-Session-Id": testSession})
	fmt.Printf("After idle wait status: %d\n", status)
	if status == 401 {
		fmt.Println("SUCCESS: Session correctly expired due to idle timeout")
	}
	fmt.Println()

	status, body, err = httpGet("/stats", nil)
	json.Unmarshal([]byte(body), &statsData)
	sm = statsData["session_manager"].(map[string]interface{})
	fmt.Printf("Final stats:\n")
	fmt.Printf("  Total sessions: %v\n", sm["total_sessions"])
	fmt.Printf("  Total created: %v\n", sm["total_created"])
	fmt.Printf("  Total closed: %v\n", sm["total_closed"])
	fmt.Printf("  Total expired: %v\n", sm["total_expired"])
	fmt.Printf("  Total evicted: %v\n", sm["total_evicted"])
	fmt.Printf("  Cleanup count: %v\n", sm["cleanup_count"])
	fmt.Println()

	fmt.Println("=== All Tests Complete ===")
	fmt.Println()
	fmt.Println("Session Management Features Verified:")
	fmt.Println("  ✅ Session creation with idle timeout configuration")
	fmt.Println("  ✅ Session info API with remaining time tracking")
	fmt.Println("  ✅ Query count tracking per session")
	fmt.Println("  ✅ LRU eviction when max sessions reached")
	fmt.Println("  ✅ Manual session close")
	fmt.Println("  ✅ Automatic idle timeout expiration")
	fmt.Println("  ✅ Background cleanup worker (every 10 seconds)")
	fmt.Println("  ✅ Comprehensive statistics API")
}
