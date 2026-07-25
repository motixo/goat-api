package routes

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestClassificationRegistryRejectsDuplicateRouteClassification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := NewClassificationRegistry()
	group := gin.New().Group("/api/v1")
	registry.Public(group, http.MethodGet, "/health", func(*gin.Context) {})

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("duplicate route classification did not panic")
		}
		message, ok := recovered.(string)
		if !ok || !strings.Contains(message, "duplicate route authorization classification") {
			t.Fatalf("panic = %#v, want duplicate-classification diagnostic", recovered)
		}
	}()

	registry.FreshIdentity(group, http.MethodGet, "/health", func(*gin.Context) {})
}
