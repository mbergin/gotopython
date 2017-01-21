package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSanity(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Error(w.Body)
	}
}

func BenchmarkWeb(b *testing.B) {
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler(w, req)
	}
}
