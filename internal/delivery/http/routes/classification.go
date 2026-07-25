package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type AuthorizationClass string

const (
	AuthorizationPublic             AuthorizationClass = "public"
	AuthorizationSnapshot           AuthorizationClass = "authenticated_snapshot"
	AuthorizationFreshIdentity      AuthorizationClass = "authenticated_fresh_identity"
	AuthorizationFreshAuthorization AuthorizationClass = "authenticated_fresh_authorization"
)

type RouteClassification struct {
	Method     string
	Path       string
	Class      AuthorizationClass
	Permission valueobject.Permission
}

type ClassificationRegistry struct {
	entries []RouteClassification
	seen    map[string]struct{}
}

func NewClassificationRegistry() *ClassificationRegistry {
	return &ClassificationRegistry{seen: make(map[string]struct{})}
}

func (r *ClassificationRegistry) Public(
	router *gin.RouterGroup,
	method string,
	path string,
	handlers ...gin.HandlerFunc,
) {
	r.register(router, method, path, AuthorizationPublic, "", handlers...)
}

func (r *ClassificationRegistry) Snapshot(
	router *gin.RouterGroup,
	method string,
	path string,
	required valueobject.Permission,
	handlers ...gin.HandlerFunc,
) {
	r.register(
		router,
		method,
		path,
		AuthorizationSnapshot,
		required,
		handlers...,
	)
}

func (r *ClassificationRegistry) FreshIdentity(
	router *gin.RouterGroup,
	method string,
	path string,
	handlers ...gin.HandlerFunc,
) {
	r.register(
		router,
		method,
		path,
		AuthorizationFreshIdentity,
		"",
		handlers...,
	)
}

func (r *ClassificationRegistry) FreshAuthorization(
	router *gin.RouterGroup,
	method string,
	path string,
	required valueobject.Permission,
	handlers ...gin.HandlerFunc,
) {
	r.register(
		router,
		method,
		path,
		AuthorizationFreshAuthorization,
		required,
		handlers...,
	)
}

func (r *ClassificationRegistry) Entries() []RouteClassification {
	return append([]RouteClassification(nil), r.entries...)
}

func (r *ClassificationRegistry) register(
	router *gin.RouterGroup,
	method string,
	path string,
	class AuthorizationClass,
	required valueobject.Permission,
	handlers ...gin.HandlerFunc,
) {
	if method == "" {
		method = http.MethodGet
	}
	key := method + " " + router.BasePath() + path
	if _, duplicate := r.seen[key]; duplicate {
		panic("duplicate route authorization classification: " + key)
	}
	r.seen[key] = struct{}{}
	r.entries = append(r.entries, RouteClassification{
		Method:     method,
		Path:       router.BasePath() + path,
		Class:      class,
		Permission: required,
	})
	router.Handle(method, path, handlers...)
}
