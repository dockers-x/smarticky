package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/user"
)

var ErrUnauthorized = errors.New("unauthorized")

type principalContextKey struct{}

type Principal struct {
	UserID   int
	Username string
	Source   string
	BaseURL  string
}

type Authenticator struct {
	client              *ent.Client
	trustLazyCatHeaders bool
}

func NewAuthenticator(client *ent.Client, trustLazyCatHeaders bool) *Authenticator {
	return &Authenticator{
		client:              client,
		trustLazyCatHeaders: trustLazyCatHeaders,
	}
}

func NewAuthenticatorFromEnv(client *ent.Client) *Authenticator {
	return NewAuthenticator(client, strings.EqualFold(os.Getenv("SMARTICKY_TRUST_LAZYCAT_HEADERS"), "true"))
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	value := ctx.Value(principalContextKey{})
	if value == nil {
		return Principal{}, false
	}
	principal, ok := value.(Principal)
	return principal, ok
}

func GenerateToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "smky_mcp_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := a.Resolve(r.Context(), r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

func (a *Authenticator) Resolve(ctx context.Context, r *http.Request) (Principal, error) {
	if a.trustLazyCatHeaders && r.Header.Get("X-HC-User-ID") != "" {
		return a.resolveLazyCat(ctx, r)
	}

	authHeader := r.Header.Get("Authorization")
	if strings.TrimSpace(authHeader) != "" {
		return a.resolveBearer(ctx, r)
	}

	return Principal{}, ErrUnauthorized
}

func (a *Authenticator) resolveLazyCat(ctx context.Context, r *http.Request) (Principal, error) {
	lazycatUID := strings.TrimSpace(r.Header.Get("X-HC-User-ID"))
	source := strings.TrimSpace(r.Header.Get("X-HC-SOURCE"))
	if lazycatUID == "" || !isTrustedLazyCatSource(source) {
		return Principal{}, ErrUnauthorized
	}

	row, err := a.client.User.Query().
		Where(user.LazycatUIDEQ(lazycatUID)).
		Only(ctx)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}

	return Principal{
		UserID:   row.ID,
		Username: row.Username,
		Source:   source,
		BaseURL:  requestBaseURL(r),
	}, nil
}

func (a *Authenticator) resolveBearer(ctx context.Context, r *http.Request) (Principal, error) {
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return Principal{}, ErrUnauthorized
	}

	hash := HashToken(fields[1])
	rows, err := a.client.MCPToken.Query().
		WithUser().
		All(ctx)
	if err != nil {
		return Principal{}, err
	}

	for _, row := range rows {
		if subtle.ConstantTimeCompare([]byte(row.TokenHash), []byte(hash)) != 1 {
			continue
		}
		owner, err := row.Edges.UserOrErr()
		if err != nil {
			return Principal{}, ErrUnauthorized
		}
		_, _ = row.Update().SetLastUsedAt(time.Now()).Save(ctx)
		return Principal{
			UserID:   owner.ID,
			Username: owner.Username,
			Source:   "token:" + strconv.Itoa(row.ID),
			BaseURL:  requestBaseURL(r),
		}, nil
	}

	return Principal{}, ErrUnauthorized
}

func isTrustedLazyCatSource(source string) bool {
	if source == "app:self" {
		return true
	}
	return strings.HasPrefix(source, "app:") && len(source) > len("app:")
}

func requestBaseURL(r *http.Request) string {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}
