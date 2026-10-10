// Package httpapi wires the REST API.
package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mileszhanghao/drizzle/backend/internal/ai"
	"github.com/mileszhanghao/drizzle/backend/internal/auth"
	"github.com/mileszhanghao/drizzle/backend/internal/media"
	"github.com/mileszhanghao/drizzle/backend/internal/store"
)

const (
	maxUploadBytes   = 50 << 20
	maxMessageLength = 2000
	maxPromptLength  = 1000
)

type Server struct {
	Store  store.Store
	Media  media.Store
	Tokens *auth.Tokens
	// Images is optional; without it the AI endpoint returns 503.
	Images ai.ImageGenerator
	// ImageQuota limits generations per user per hour (cost control).
	ImageQuota int
	Log        *slog.Logger
	Now        func() time.Time

	quotaMu sync.Mutex
	quota   map[string][]time.Time
}

// Routes returns the HTTP handler. It uses Go 1.22+ method-and-path patterns
// from the standard library instead of a third-party router.
func (s *Server) Routes() http.Handler {
	if s.Now == nil {
		s.Now = time.Now
	}
	if s.Log == nil {
		s.Log = slog.Default()
	}
	requireAuth := s.Tokens.Require

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/signup", s.signup)
	mux.HandleFunc("POST /api/signin", s.signin)
	mux.Handle("GET /api/posts", requireAuth(http.HandlerFunc(s.listPosts)))
	mux.Handle("POST /api/posts", requireAuth(http.HandlerFunc(s.createPost)))
	mux.Handle("DELETE /api/posts/{id}", requireAuth(http.HandlerFunc(s.deletePost)))
	mux.Handle("POST /api/ai/images", requireAuth(http.HandlerFunc(s.generateImage)))
	mux.Handle("GET /media/{key}", s.Media.Handler())
	return mux
}

type postJSON struct {
	store.Post
	MediaURL string `json:"mediaUrl"`
}

func toJSON(p store.Post) postJSON { return postJSON{Post: p, MediaURL: "/media/" + p.MediaKey} }

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "imageGeneration": s.Images != nil})
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decodeJSON(w, r, &c) {
		return
	}
	c.Username = strings.ToLower(strings.TrimSpace(c.Username))
	if err := auth.ValidateCredentials(c.Username, c.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(c.Password)
	if err != nil {
		s.fail(w, err)
		return
	}
	switch err := s.Store.CreateUser(r.Context(), c.Username, hash); {
	case errors.Is(err, store.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "username already taken")
	case err != nil:
		s.fail(w, err)
	default:
		writeJSON(w, http.StatusCreated, map[string]string{"username": c.Username})
	}
}

func (s *Server) signin(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decodeJSON(w, r, &c) {
		return
	}
	c.Username = strings.ToLower(strings.TrimSpace(c.Username))
	user, err := s.Store.GetUser(r.Context(), c.Username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.fail(w, err)
		return
	}
	ok := false
	if err == nil {
		ok, _ = auth.VerifyPassword(c.Password, user.PasswordHash)
	}
	if !ok {
		// Same message for unknown user and wrong password.
		writeError(w, http.StatusUnauthorized, "wrong username or password")
		return
	}
	token, err := s.Tokens.Issue(user.Username)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "username": user.Username})
}

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	posts, err := s.Store.ListPosts(r.Context(), store.Query{
		Text:     r.URL.Query().Get("q"),
		Username: strings.ToLower(r.URL.Query().Get("user")),
		Limit:    limit,
	})
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]postJSON, len(posts))
	for i, p := range posts {
		out[i] = toJSON(p)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "file is larger than 50 MB")
			return
		}
		writeError(w, http.StatusBadRequest, "expected a multipart form")
		return
	}
	defer r.MultipartForm.RemoveAll()

	message := strings.TrimSpace(r.FormValue("message"))
	if len([]rune(message)) > maxMessageLength {
		writeError(w, http.StatusBadRequest, "message is longer than 2000 characters")
		return
	}
	file, _, err := r.FormFile("media")
	if err != nil {
		writeError(w, http.StatusBadRequest, `attach an image or video in the "media" field`)
		return
	}
	defer file.Close()

	body, kind, ext, err := media.Sniff(file)
	if errors.Is(err, media.ErrUnsupported) {
		writeError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}

	post := store.Post{
		ID:        store.NewID(),
		Username:  auth.Username(r.Context()),
		Message:   message,
		MediaType: kind,
		CreatedAt: s.Now().UTC(),
	}
	post.MediaKey = post.ID + ext
	if err := s.Media.Put(r.Context(), post.MediaKey, body); err != nil {
		s.fail(w, err)
		return
	}
	if err := s.Store.CreatePost(r.Context(), post); err != nil {
		_ = s.Media.Delete(r.Context(), post.MediaKey) // don't leave orphaned files
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toJSON(post))
}

func (s *Server) deletePost(w http.ResponseWriter, r *http.Request) {
	key, err := s.Store.DeletePost(r.Context(), r.PathValue("id"), auth.Username(r.Context()))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.Media.Delete(r.Context(), key); err != nil {
		s.Log.Warn("post deleted but media file remained", "key", key, "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// generateImage returns a preview as a data URL. The user then posts it like
// any other upload, so generated images go through the same validation.
func (s *Server) generateImage(w http.ResponseWriter, r *http.Request) {
	if s.Images == nil {
		writeError(w, http.StatusServiceUnavailable, "image generation is not configured on this server")
		return
	}
	var req struct {
		Prompt string `json:"prompt"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" || len([]rune(req.Prompt)) > maxPromptLength {
		writeError(w, http.StatusBadRequest, "prompt must be 1-1000 characters")
		return
	}
	if !s.takeQuota(auth.Username(r.Context())) {
		writeError(w, http.StatusTooManyRequests, "image generation limit reached, try again later")
		return
	}
	img, err := s.Images.Generate(r.Context(), req.Prompt)
	if err != nil {
		s.Log.Error("image generation failed", "err", err)
		writeError(w, http.StatusBadGateway, "image generation failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"dataUrl": "data:image/png;base64," + base64.StdEncoding.EncodeToString(img),
	})
}

// takeQuota is a per-user sliding one-hour window.
func (s *Server) takeQuota(user string) bool {
	if s.ImageQuota <= 0 {
		return true
	}
	s.quotaMu.Lock()
	defer s.quotaMu.Unlock()
	if s.quota == nil {
		s.quota = map[string][]time.Time{}
	}
	now := s.Now()
	recent := s.quota[user][:0]
	for _, t := range s.quota[user] {
		if now.Sub(t) < time.Hour {
			recent = append(recent, t)
		}
	}
	if len(recent) >= s.ImageQuota {
		s.quota[user] = recent
		return false
	}
	s.quota[user] = append(recent, now)
	return true
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.Log.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "something went wrong")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
