package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIImages(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nfake")
	var gotAuth, gotPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var req imagesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotPrompt = req.Prompt
		if req.Prompt == "fail" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"prompt rejected"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(png)}},
		})
	}))
	defer srv.Close()

	gen := NewOpenAIImages("sk-test", "gpt-image-1")
	gen.BaseURL = srv.URL

	img, err := gen.Generate(context.Background(), "a cat in Seattle rain")
	if err != nil {
		t.Fatal(err)
	}
	if string(img) != string(png) || gotAuth != "Bearer sk-test" || gotPrompt != "a cat in Seattle rain" {
		t.Fatalf("unexpected call: auth=%q prompt=%q img=%q", gotAuth, gotPrompt, img)
	}

	if _, err := gen.Generate(context.Background(), "fail"); err == nil || !strings.Contains(err.Error(), "prompt rejected") {
		t.Fatalf("expected API error message, got %v", err)
	}
}
