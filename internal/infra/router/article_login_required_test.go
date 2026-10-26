package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/internal/app/middleware"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/auth"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	articlehandler "github.com/anzhiyu-c/anheyu-app/pkg/handler/article"
	articleservice "github.com/anzhiyu-c/anheyu-app/pkg/service/article"
	authservice "github.com/anzhiyu-c/anheyu-app/pkg/service/auth"
	"github.com/gin-gonic/gin"
)

type loginReadingTokens struct{ authservice.TokenService }

func (loginReadingTokens) ParseAccessToken(_ context.Context, token string) (*auth.CustomClaims, error) {
	if token != "valid-reader" {
		return nil, errors.New("expired token")
	}
	return &auth.CustomClaims{UserID: "ordinary-reader"}, nil
}

type loginReadingService struct{ articleservice.Service }

func (loginReadingService) GetLoginRequiredContent(_ context.Context, id string) (string, error) {
	if id != "published-post" {
		return "", constant.ErrNotFound
	}
	return "<p>private-canary</p>", nil
}

func TestLoginRequiredReadingRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := &Router{
		mw:             middleware.NewMiddleware(loginReadingTokens{}),
		articleHandler: articlehandler.NewHandler(loginReadingService{}),
	}
	engine := gin.New()
	api := engine.Group("/api", NoCacheMiddleware())
	router.registerArticleRoutes(api)
	for _, tc := range []struct {
		name, id, authorization string
		status                  int
		secret                  bool
	}{
		{"anonymous", "published-post", "", 401, false},
		{"forged token", "published-post", "Bearer forged", 401, false},
		{"reader", "published-post", "Bearer valid-reader", 200, true},
		{"unpublished or missing", "draft-post", "Bearer valid-reader", 404, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/articles/"+tc.id+"/login-required-content", nil)
			req.Header.Set("Authorization", tc.authorization)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if strings.Contains(rec.Body.String(), "private-canary") != tc.secret {
				t.Fatal("protected response has incorrect visibility")
			}
			if cache := rec.Header().Get("Cache-Control"); !strings.Contains(cache, "no-store") || !strings.Contains(cache, "private") {
				t.Errorf("protected response is cacheable: %q", cache)
			}
		})
	}
}
