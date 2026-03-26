package agui

import (
	"encoding/json"
	"errors"
	"net/http"
)

type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming is not supported by this response writer")
	}
	return &SSEWriter{w: w, flusher: flusher}, nil
}

func (s *SSEWriter) Open() {
	s.w.Header().Set("Content-Type", "text/event-stream")
	s.w.Header().Set("Cache-Control", "no-cache")
	s.w.Header().Set("Connection", "keep-alive")
	_, _ = s.w.Write([]byte("retry: 1000\n\n"))
	s.flusher.Flush()
}

func (s *SSEWriter) Write(event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := s.w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
