package main

import (
	"io"
	"log"
	"net/http"
	"strings"
)

const serverAddr = ":8787"

func runServer() {
	queue := make(chan string, 16)
	go ttsWorker(queue)

	http.HandleFunc("/tts", ttsHandler(queue))
	log.Printf("tts server listening on %s", serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}

func ttsWorker(queue <-chan string) {
	for text := range queue {
		if err := speak(text); err != nil {
			log.Printf("tts: %v", err)
		}
	}
}

func ttsHandler(queue chan<- string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s from %s (origin=%q)", r.Method, r.URL.Path, r.RemoteAddr, r.Header.Get("Origin"))
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		h.Set("Access-Control-Allow-Private-Network", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
			return
		}
		text := strings.TrimSpace(string(body))
		if text == "" {
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}
		select {
		case queue <- text:
			log.Printf("tts: %s", text)
			w.WriteHeader(http.StatusAccepted)
		default:
			log.Printf("tts queue full — dropping %q", text)
			http.Error(w, "queue full", http.StatusServiceUnavailable)
		}
	}
}
