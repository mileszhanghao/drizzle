package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mileszhanghao/drizzle/backend/internal/auth"
	"github.com/mileszhanghao/drizzle/backend/internal/media"
	"github.com/mileszhanghao/drizzle/backend/internal/store"
)

// A real 1x1 PNG so content sniffing sees genuine image bytes.
var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82")

type fakeImages struct{ err error }

func (f fakeImages) Generate(context.Context, string) ([]byte, error) { return pngBytes, f.err }

type harness struct {
	t        *testing.T
	srv      *httptest.Server
	mediaDir string
}

func newHarness(t *testing.T, images *fakeImages) *harness {
	t.Helper()
	tokens, _ := auth.NewTokens("test-secret-that-is-at-least-32-bytes-long", time.Hour)
	dir := t.TempDir()
	local, _ := media.NewLocal(dir)
	s := &Server{Store: store.NewMemory(), Media: local, Tokens: tokens, ImageQuota: 2}
	if images != nil {
		s.Images = images
	}
	srv := httptest.NewServer(s.Routes())
	t.Cleanup(srv.Close)
	return &harness{t: t, srv: srv, mediaDir: dir}
}

func (h *harness) do(method, path, token string, body io.Reader, contentType string) (int, map[string]any, []byte) {
	h.t.Helper()
	req, _ := http.NewRequest(method, h.srv.URL+path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var obj map[string]any
	_ = json.Unmarshal(raw, &obj)
	return resp.StatusCode, obj, raw
}

func (h *harness) jsonReq(method, path, token string, v any) (int, map[string]any, []byte) {
	b, _ := json.Marshal(v)
	return h.do(method, path, token, bytes.NewReader(b), "application/json")
}

func (h *harness) signUpAndIn(user string) string {
	h.t.Helper()
	if code, _, raw := h.jsonReq("POST", "/api/signup", "", credentials{user, "password123"}); code != 201 {
		h.t.Fatalf("signup %s: %d %s", user, code, raw)
	}
	code, body, raw := h.jsonReq("POST", "/api/signin", "", credentials{user, "password123"})
	if code != 200 {
		h.t.Fatalf("signin %s: %d %s", user, code, raw)
	}
	return body["token"].(string)
}

func (h *harness) post(token, message string, file []byte) (int, map[string]any, []byte) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("message", message)
	fw, _ := mw.CreateFormFile("media", "photo.jpg") // name/extension deliberately misleading
	_, _ = fw.Write(file)
	_ = mw.Close()
	return h.do("POST", "/api/posts", token, &buf, mw.FormDataContentType())
}

func TestSignupAndSignin(t *testing.T) {
	h := newHarness(t, nil)
	h.signUpAndIn("miles")

	if code, _, _ := h.jsonReq("POST", "/api/signup", "", credentials{"miles", "otherpassword"}); code != http.StatusConflict {
		t.Fatalf("duplicate signup: %d", code)
	}
	if code, _, _ := h.jsonReq("POST", "/api/signup", "", credentials{"Bad Name!", "password123"}); code != http.StatusBadRequest {
		t.Fatalf("invalid username: %d", code)
	}
	if code, _, _ := h.jsonReq("POST", "/api/signin", "", credentials{"miles", "wrongpassword"}); code != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", code)
	}
	if code, _, _ := h.jsonReq("POST", "/api/signin", "", credentials{"nobody", "password123"}); code != http.StatusUnauthorized {
		t.Fatalf("unknown user: %d", code)
	}
}

func TestPostsRequireAuth(t *testing.T) {
	h := newHarness(t, nil)
	if code, _, _ := h.do("GET", "/api/posts", "", nil, ""); code != http.StatusUnauthorized {
		t.Fatalf("got %d", code)
	}
	if code, _, _ := h.do("GET", "/api/posts", "not-a-token", nil, ""); code != http.StatusUnauthorized {
		t.Fatalf("bad token: got %d", code)
	}
}

func TestCreateSearchServeAndDeletePost(t *testing.T) {
	h := newHarness(t, nil)
	alice := h.signUpAndIn("alice")
	bob := h.signUpAndIn("bob")

	code, created, raw := h.post(alice, "Sunrise hiking at Rainier", pngBytes)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, raw)
	}
	if created["username"] != "alice" || created["mediaType"] != "image" {
		t.Fatalf("unexpected post: %v", created)
	}
	mediaURL := created["mediaUrl"].(string)
	if !strings.HasSuffix(mediaURL, ".png") {
		t.Fatalf("media key should use the sniffed type (.png), got %s", mediaURL)
	}

	// Media is served with the right content type.
	resp, err := http.Get(h.srv.URL + mediaURL)
	if err != nil || resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("serve media: %v %d %s", err, resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp.Body.Close()

	// Search.
	_, _, list := h.do("GET", "/api/posts?q=hiking", bob, nil, "")
	var posts []map[string]any
	_ = json.Unmarshal(list, &posts)
	if len(posts) != 1 || posts[0]["message"] != "Sunrise hiking at Rainier" {
		t.Fatalf("search: %s", list)
	}
	_, _, none := h.do("GET", "/api/posts?user=bob", bob, nil, "")
	if strings.TrimSpace(string(none)) != "[]" {
		t.Fatalf("bob has no posts, got %s", none)
	}

	// Only the owner can delete.
	id := created["id"].(string)
	if code, _, _ := h.do("DELETE", "/api/posts/"+id, bob, nil, ""); code != http.StatusNotFound {
		t.Fatalf("bob deleting alice's post: %d", code)
	}
	if code, _, _ := h.do("DELETE", "/api/posts/"+id, alice, nil, ""); code != http.StatusNoContent {
		t.Fatalf("alice deleting her post: %d", code)
	}
	if _, err := os.Stat(filepath.Join(h.mediaDir, strings.TrimPrefix(mediaURL, "/media/"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("media file should be removed with the post, stat err = %v", err)
	}
}

func TestUploadValidation(t *testing.T) {
	h := newHarness(t, nil)
	token := h.signUpAndIn("miles")

	// A text file named photo.jpg: the course version accepted it based on the extension.
	if code, _, _ := h.post(token, "hi", []byte("definitely not an image")); code != http.StatusUnsupportedMediaType {
		t.Fatalf("fake image: got %d", code)
	}
	if code, _, _ := h.post(token, strings.Repeat("x", 2001), pngBytes); code != http.StatusBadRequest {
		t.Fatalf("long message: got %d", code)
	}
	if code, _, _ := h.do("GET", "/media/..%2f..%2fetc%2fpasswd", "", nil, ""); code != http.StatusNotFound {
		t.Fatalf("path traversal: got %d", code)
	}
}

func TestImageGeneration(t *testing.T) {
	disabled := newHarness(t, nil)
	tok := disabled.signUpAndIn("miles")
	if code, _, _ := disabled.jsonReq("POST", "/api/ai/images", tok, map[string]string{"prompt": "a cat"}); code != http.StatusServiceUnavailable {
		t.Fatalf("not configured: got %d", code)
	}

	h := newHarness(t, &fakeImages{})
	tok = h.signUpAndIn("miles")
	code, body, raw := h.jsonReq("POST", "/api/ai/images", tok, map[string]string{"prompt": "a cat in the rain"})
	if code != 200 || !strings.HasPrefix(body["dataUrl"].(string), "data:image/png;base64,") {
		t.Fatalf("generate: %d %s", code, raw)
	}
	// Quota is 2 per hour in this harness.
	h.jsonReq("POST", "/api/ai/images", tok, map[string]string{"prompt": "again"})
	if code, _, _ := h.jsonReq("POST", "/api/ai/images", tok, map[string]string{"prompt": "third"}); code != http.StatusTooManyRequests {
		t.Fatalf("quota: got %d", code)
	}
	if code, _, _ := h.jsonReq("POST", "/api/ai/images", tok, map[string]string{"prompt": ""}); code != http.StatusBadRequest {
		t.Fatalf("empty prompt: got %d", code)
	}

	failing := newHarness(t, &fakeImages{err: errors.New("upstream down")})
	tok = failing.signUpAndIn("miles")
	if code, _, _ := failing.jsonReq("POST", "/api/ai/images", tok, map[string]string{"prompt": "x"}); code != http.StatusBadGateway {
		t.Fatalf("upstream failure: got %d", code)
	}
}
