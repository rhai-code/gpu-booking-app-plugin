package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type UserInfo struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
	IsAdmin  bool     `json:"is_admin"`
}

type contextKey string

const userContextKey contextKey = "user"

type cachedUser struct {
	info      *UserInfo
	expiresAt time.Time
}

var (
	authCache   sync.Map
	authClient  *kubernetes.Clientset
	authInitErr error
	authOnce    sync.Once
)

func initAuthClient() {
	authOnce.Do(func() {
		config, err := rest.InClusterConfig()
		if err != nil {
			authInitErr = err
			log.Printf("Warning: auth client not available (RBAC won't work): %v", err)
			return
		}
		authClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			authInitErr = err
			log.Printf("Warning: auth client not available (RBAC won't work): %v", err)
		}
	})
}

func GetUser(r *http.Request) *UserInfo {
	if user, ok := r.Context().Value(userContextKey).(*UserInfo); ok {
		return user
	}
	return &UserInfo{Username: "", IsAdmin: false}
}

func AuthMiddleware(next http.Handler) http.Handler {
	initAuthClient()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health endpoints
		if strings.HasSuffix(r.URL.Path, "/health") || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		// Internal pod-to-pod requests (e.g. agent/MCP tool -> backend) bypass token auth
		if r.Header.Get("X-Internal-Request") == "true" {
			username := r.Header.Get("X-Forwarded-User")
			if username == "" {
				username = "internal"
			}
			ctx := context.WithValue(r.Context(), userContextKey, &UserInfo{Username: username, IsAdmin: false})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			// If no auth client available, allow anonymous access (dev mode)
			if authClient == nil {
				ctx := context.WithValue(r.Context(), userContextKey, &UserInfo{Username: "anonymous", IsAdmin: true})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			HttpError(w, http.StatusUnauthorized, "authorization required")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Check cache
		hash := sha256.Sum256([]byte(token))
		cacheKey := hex.EncodeToString(hash[:])

		if cached, ok := authCache.Load(cacheKey); ok {
			cu := cached.(*cachedUser)
			if time.Now().Before(cu.expiresAt) {
				ctx := context.WithValue(r.Context(), userContextKey, cu.info)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			authCache.Delete(cacheKey)
		}

		if authClient == nil {
			ctx := context.WithValue(r.Context(), userContextKey, &UserInfo{Username: "anonymous", IsAdmin: true})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// TokenReview
		tr, err := authClient.AuthenticationV1().TokenReviews().Create(r.Context(), &authenticationv1.TokenReview{
			Spec: authenticationv1.TokenReviewSpec{Token: token},
		}, metav1.CreateOptions{})
		if err != nil || !tr.Status.Authenticated {
			log.Printf("TokenReview failed for token (first 20 chars): %s... err=%v authenticated=%v",
				token[:min(20, len(token))], err, tr != nil && tr.Status.Authenticated)
			HttpError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		username := tr.Status.User.Username
		groups := tr.Status.User.Groups

		// SubjectAccessReview for admin check
		sar, err := authClient.AuthorizationV1().SubjectAccessReviews().Create(r.Context(), &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				User:   username,
				Groups: groups,
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Group:    "gpubooking.openshift.io",
					Resource: "bookings",
					Verb:     "admin",
				},
			},
		}, metav1.CreateOptions{})
		isAdmin := err == nil && sar.Status.Allowed

		user := &UserInfo{
			Username: username,
			Groups:   groups,
			IsAdmin:  isAdmin,
		}

		// Cache for 60 seconds
		authCache.Store(cacheKey, &cachedUser{
			info:      user,
			expiresAt: time.Now().Add(60 * time.Second),
		})

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func MeHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	JsonResponse(w, user)
}
