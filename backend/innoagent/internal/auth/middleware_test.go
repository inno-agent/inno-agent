package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_InjectsUserIDOnValidToken(t *testing.T) {
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"u1"}`))
	}))
	defer idp.Close()

	var got string
	h := Middleware(NewClient(idp.URL))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer xyz")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if got != "u1" {
		t.Fatalf("user_id not injected: %q", got)
	}
}

func TestMiddleware_RejectsMissingBearer(t *testing.T) {
	h := Middleware(NewClient("http://unused"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestMiddleware_RejectsInvalidToken(t *testing.T) {
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
	}))
	defer idp.Close()

	h := Middleware(NewClient(idp.URL))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer bad")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestMiddleware_ServiceToken_WithUserID_SetsUserID(t *testing.T) {
	identitySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"svc:review-consumer"}`))
	}))
	defer identitySrv.Close()

	var gotUserID string
	h := Middleware(NewClient(identitySrv.URL))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer svc-token")
	req.Header.Set("X-User-ID", "real-user-uuid")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotUserID != "real-user-uuid" {
		t.Fatalf("expected real-user-uuid, got %q", gotUserID)
	}
}

func TestMiddleware_ServiceToken_MissingUserID_400(t *testing.T) {
	identitySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"svc:review-consumer"}`))
	}))
	defer identitySrv.Close()

	h := Middleware(NewClient(identitySrv.URL))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer svc-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMiddleware_UserToken_XUserIDIgnored(t *testing.T) {
	identitySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"real-user-uuid"}`))
	}))
	defer identitySrv.Close()

	var gotUserID string
	h := Middleware(NewClient(identitySrv.URL))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer user-token")
	req.Header.Set("X-User-ID", "attacker-uuid")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if gotUserID != "real-user-uuid" {
		t.Fatalf("X-User-ID must be ignored for user tokens; got %q", gotUserID)
	}
}
