// fake-lark — a minimal Lark webhook receiver for local testing.
// Listens on :9999 and prints incoming card payloads.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	port := "9999"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("📥 POST %s\n", r.URL.Path)

		// Pretty-print if JSON, otherwise raw
		var parsed any
		if json.Unmarshal(body, &parsed) == nil {
			pretty, _ := json.MarshalIndent(parsed, "", "  ")
			fmt.Println(string(pretty))
		} else {
			fmt.Println(string(body))
		}

		// Simulate Lark API response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"StatusCode": 0,
			"code":       0,
			"msg":        "success",
		})
		fmt.Println("✅ 200 OK — success")
	})

	log.Printf("🟢 Fake Lark listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
