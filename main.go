package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type queue chan string

type QueueStore struct {
	sync.RWMutex
	queues map[string]queue
}

func NewStorage() *QueueStore {
	return &QueueStore{
		queues: make(map[string]queue),
	}
}

func (s *QueueStore) handlePut(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path
	message := r.URL.Query().Get("v")
	if message == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.Lock()
	if _, ok := s.queues[name]; !ok {
		s.queues[name] = make(queue)
	}
	s.Unlock()
	go func() {
		s.RLock()
		queue := s.queues[name]
		s.RUnlock()
		queue <- message
	}()
}

func (s *QueueStore) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path

	var timeout int
	timeoutStr := r.URL.Query().Get("timeout")
	if timeoutStr != "" {
		var err error
		timeout, err = strconv.Atoi(timeoutStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	s.RLock()
	queue := s.queues[name]
	s.RUnlock()

	if timeout == 0 {
		select {
		case m := <-queue:
			_, err := io.WriteString(w, m)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
		return
	}

	select {
	case m := <-queue:
		_, err := io.WriteString(w, m)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case <-time.After(time.Duration(timeout) * time.Second):
		w.WriteHeader(http.StatusNotFound)
	}
}

func getHandler() http.HandlerFunc {
	s := NewStorage()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			s.handlePut(w, r)
		case http.MethodGet:
			s.handleGet(w, r)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}

func main() {
	http.HandleFunc("/", getHandler())

	if len(os.Args) < 2 {
		log.Fatal("port number is required")
	}
	port := os.Args[1]

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
