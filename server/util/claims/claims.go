package claims

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/buildbuddy-io/buildbuddy/server/environment"
	"github.com/buildbuddy-io/buildbuddy/server/interfaces"
	"github.com/buildbuddy-io/buildbuddy/server/tables"
	"github.com/buildbuddy-io/buildbuddy/server/util/authutil"
	"github.com/buildbuddy-io/buildbuddy/server/util/capabilities"
	"github.com/buildbuddy-io/buildbuddy/server/util/flag"
	"github.com/buildbuddy-io/buildbuddy/server/util/log"
	"github.com/buildbuddy-io/buildbuddy/server/util/lru"
	"github.com/buildbuddy-io/buildbuddy/server/util/role"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"github.com/buildbuddy-io/buildbuddy/server/util/subdomain"
	"github.com/golang-jwt/jwt/v4"

	cappb "github.com/buildbuddy-io/buildbuddy/proto/capability"
	requestcontext "github.com/buildbuddy-io/buildbuddy/server/util/request_context"
)

const (
	// Maximum number of entries in JWT -> Claims cache.
	claimsCacheSize = 10_00

	// The key the Claims are stored under in the context.
	// If unset, the JWT can be used to reconstitute the claims.
	contextClaimsKey = "auth.claims"
)

var (
	jwtKey             = flag.String("auth.jwt_key", "set_the_jwt_in_config", "The key to use when signing JWT tokens.", flag.Secret)
	newJwtKey          = flag.String("auth.new_jwt_key", "", "If set, JWT verifications will try both this and the old JWT key.", flag.Secret)
	signUsingNewJwtKey = flag.Bool("auth.sign_using_new_jwt_key", false, "If true, new JWTs will be signed using the new JWT key.")
	claimsCacheTTL     = flag.Duration("auth.jwt_claims_cache_ttl", 15*time.Second, "TTL for JWT string to parsed claims caching. Set to '0' to disable cache.")
	jwtDuration        = flag.Duration("auth.jwt_duration", 6*time.Hour, "Maximum lifetime of the generated JWT.")
)

type Claims struct {
	jwt.StandardClaims
	APIKeyID string `json:"api_key_id,omitempty"`
	UserID   string `json:"user_id"`
	GroupID  string `json:"group_id"`
	// APIKeyOwnerGroupID identifies the group that owns the API key used
	// for authentication. Will be empty if authentication was not performed
	// using an API key.
	// The GroupID field above should be used in most cases!
	APIKeyOwnerGroupID string `json:"api_key_owner_group_id,omitempty"`
	Impersonating      bool   `json:"impersonating"`
	// TODO(bduffany): remove this field
	AllowedGroups          []string                      `json:"allowed_groups"`
	GroupMemberships       []*interfaces.GroupMembership `json:"group_memberships"`
	Capabilities           []cappb.Capability            `json:"capabilities"`
	UseGroupOwnedExecutors bool                          `json:"use_group_owned_executors,omitempty"`
	CacheEncryptionEnabled bool                          `json:"cache_encryption_enabled,omitempty"`
	EnforceIPRules         bool                          `json:"enforce_ip_rules,omitempty"`
	// TODO(vadim): remove this field
	SAML        bool `json:"saml,omitempty"`
	CustomerSSO bool `json:"customer_sso,omitempty"`
}

func (c *Claims) GetAPIKeyInfo() interfaces.APIKeyInfo {
	return interfaces.APIKeyInfo{
		ID: c.APIKeyID, OwnerGroupID: c.APIKeyOwnerGroupID,
	}
}

func (c *Claims) GetUserID() string {
	return c.UserID
}

func (c *Claims) GetGroupID() string {
	return c.GroupID
}

func (c *Claims) IsImpersonating() bool {
	return c.Impersonating
}

func (c *Claims) GetAllowedGroups() []string {
	return c.AllowedGroups
}

func (c *Claims) GetGroupMemberships() []*interfaces.GroupMembership {
	return c.GroupMemberships
}

func (c *Claims) GetCapabilities() []cappb.Capability {
	return c.Capabilities
}

func (c *Claims) HasCapability(cap cappb.Capability) bool {
	for _, cc := range c.Capabilities {
		if cap&cc > 0 {
			return true
		}
	}
	return false
}

func (c *Claims) GetUseGroupOwnedExecutors() bool {
	return c.UseGroupOwnedExecutors
}

func (c *Claims) GetCacheEncryptionEnabled() bool {
	return c.CacheEncryptionEnabled
}

func (c *Claims) GetEnforceIPRules() bool {
	return c.EnforceIPRules
}

func (c *Claims) IsSAML() bool {
	return c.SAML
}

func (c *Claims) IsCustomerSSO() bool {
	return c.SAML || c.CustomerSSO
}

func ParseClaims(token string) (*Claims, error) {
	keys := []string{*jwtKey}
	if *newJwtKey != "" {
		// Try the new key first.
		keys = []string{*newJwtKey, *jwtKey}
	}

	var lastErr error
	claims := &Claims{}
	for _, key := range keys {
		_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(key), nil
		})
		if err == nil {
			return claims, nil
		}
		lastErr = err

		var validationErr *jwt.ValidationError
		if errors.As(err, &validationErr) && validationErr.Errors&jwt.ValidationErrorSignatureInvalid != 0 {
			continue
		}
		return nil, err
	}
	return nil, lastErr
}

func APIKeyGroupClaims(ctx context.Context, akg interfaces.APIKeyGroup) (*Claims, error) {
	keyRole := role.Default
	// User management through SCIM requires Admin access.
	if akg.GetCapabilities()&int32(cappb.Capability_ORG_ADMIN) > 0 {
		keyRole = role.Admin
	}
	allowedGroups := []string{akg.GetGroupID()}
	groupMemberships := []*interfaces.GroupMembership{{
		GroupID:      akg.GetGroupID(),
		Capabilities: capabilities.FromInt(akg.GetCapabilities()),
		Role:         keyRole,
	}}
	for _, cg := range akg.GetChildGroupIDs() {
		allowedGroups = append(allowedGroups, cg)
		groupMemberships = append(groupMemberships, &interfaces.GroupMembership{
			GroupID:      cg,
			Capabilities: capabilities.FromInt(akg.GetCapabilities()),
			Role:         keyRole,
		})
	}

	requestContext := requestcontext.ProtoRequestContextFromContext(ctx)
	effectiveGroup := akg.GetGroupID()
	if requestContext.GetGroupId() != "" {
		if slices.Contains(allowedGroups, requestContext.GetGroupId()) {
			effectiveGroup = requestContext.GetGroupId()
		} else {
			return nil, status.PermissionDeniedErrorf("invalid group id %s", requestContext.GetGroupId())
		}
	}

	return &Claims{
		APIKeyID:               akg.GetAPIKeyID(),
		UserID:                 akg.GetUserID(),
		GroupID:                effectiveGroup,
		APIKeyOwnerGroupID:     akg.GetGroupID(),
		AllowedGroups:          allowedGroups,
		GroupMemberships:       groupMemberships,
		Capabilities:           capabilities.FromInt(akg.GetCapabilities()),
		UseGroupOwnedExecutors: akg.GetUseGroupOwnedExecutors(),
		CacheEncryptionEnabled: akg.GetCacheEncryptionEnabled(),
		EnforceIPRules:         akg.GetEnforceIPRules(),
	}, nil
}

func ClaimsFromSubID(ctx context.Context, env environment.Env, subID string) (*Claims, error) {
	authDB := env.GetAuthDB()
	if authDB == nil {
		return nil, status.FailedPreconditionError("AuthDB not configured")
	}
	u, err := authDB.LookupUserFromSubID(ctx, subID)
	if err != nil {
		return nil, err
	}

	requestContext := requestcontext.ProtoRequestContextFromContext(ctx)
	// TODO(https://github.com/buildbuddy-io/buildbuddy-internal/issues/4191):
	// return an error here once we have a better understanding of why the
	// request context can be missing.
	if requestContext == nil {
		log.CtxInfof(ctx, "Request is missing request context")
	} else if requestContext.GetGroupId() == "" {
		log.CtxInfof(ctx, "Request context group ID is empty")
	}

	eg := ""
	if requestContext.GetGroupId() != "" {
		for _, g := range u.Groups {
			if g.Group.GroupID == requestContext.GetGroupId() {
				eg = requestContext.GetGroupId()
			}
		}
	}

	claims, err := userClaims(u, eg)
	if err != nil {
		return nil, err
	}

	// If the user is trying to impersonate a member of another org and has Admin
	// role within the configured admin group, set their authenticated user to
	// *only* have access to the org being impersonated.
	if requestContext.GetImpersonatingGroupId() != "" {
		for _, membership := range claims.GetGroupMemberships() {
			if membership.GroupID != env.GetAuthenticator().AdminGroupID() || membership.Role != role.Admin {
				continue
			}

			ig, err := env.GetUserDB().GetGroupByID(ctx, requestContext.GetImpersonatingGroupId())
			if err != nil {
				return nil, err
			}

			// If the user requested impersonation but the subdomain doesn't
			// match the impersonation target then don't enable impersonation.
			if sd := subdomain.Get(ctx); sd != "" && sd != ig.URLIdentifier {
				return claims, nil
			}

			u.Groups = []*tables.GroupRole{{
				Group: *ig,
				Role:  uint32(role.Admin),
			}}
			claims, err := userClaims(u, requestContext.GetImpersonatingGroupId())
			if err != nil {
				return nil, err
			}
			claims.Impersonating = true
			return claims, nil
		}
		return nil, status.PermissionDeniedError("You do not have permissions to impersonate group members.")
	}

	return claims, nil
}

func userClaims(u *tables.User, effectiveGroup string) (*Claims, error) {
	allowedGroups := make([]string, 0, len(u.Groups))
	groupMemberships := make([]*interfaces.GroupMembership, 0, len(u.Groups))
	cacheEncryptionEnabled := false
	enforceIPRules := false
	var capabilities []cappb.Capability
	for _, g := range u.Groups {
		allowedGroups = append(allowedGroups, g.Group.GroupID)
		c, err := role.ToCapabilities(role.Role(g.Role))
		if err != nil {
			return nil, err
		}
		groupMemberships = append(groupMemberships, &interfaces.GroupMembership{
			GroupID:      g.Group.GroupID,
			Capabilities: c,
			Role:         role.Role(g.Role),
		})
		if g.Group.GroupID == effectiveGroup {
			// TODO: move these fields into u.GroupMemberships
			cacheEncryptionEnabled = g.Group.CacheEncryptionEnabled
			enforceIPRules = g.Group.EnforceIPRules
			capabilities = c
		}
	}
	return &Claims{
		UserID:                 u.UserID,
		GroupMemberships:       groupMemberships,
		AllowedGroups:          allowedGroups,
		GroupID:                effectiveGroup,
		Capabilities:           capabilities,
		CacheEncryptionEnabled: cacheEncryptionEnabled,
		EnforceIPRules:         enforceIPRules,
	}, nil
}

func AssembleJWT(c *Claims) (string, error) {
	expirationTime := time.Now().Add(*jwtDuration)
	expiresAt := expirationTime.Unix()
	// Round expiration times down to the nearest minute to improve stability
	// of JWTs for caching purposes.
	expiresAt -= (expiresAt % 60)
	c.StandardClaims = jwt.StandardClaims{ExpiresAt: expiresAt}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	key := *jwtKey
	if *newJwtKey != "" && *signUsingNewJwtKey {
		key = *newJwtKey
	}
	tokenString, err := token.SignedString([]byte(key))
	return tokenString, err
}

// Returns a context containing auth state for the provided Claims and auth
// error. Note that this function assembles a JWT out of the provided Claims
// and sets that in the context as well, so it should only be used in cases
// where that is necessary.
func AuthContextWithJWT(ctx context.Context, c *Claims, err error) context.Context {
	if err != nil {
		return authutil.AuthContextWithError(ctx, err)
	}
	tokenString, err := AssembleJWT(c)
	if err != nil {
		return authutil.AuthContextWithError(ctx, err)
	}
	ctx = context.WithValue(ctx, authutil.ContextTokenStringKey, tokenString)
	return AuthContext(ctx, c)
}

// Returns a Context containing auth state for the the provided Claims.
func AuthContext(ctx context.Context, c *Claims) context.Context {
	ctx = context.WithValue(ctx, contextClaimsKey, c)
	// Note: we clear the error here in case it was set initially by the
	// authentication handler, but then we want to re-authenticate later on in the
	// request lifecycle, and authentication is successful.
	// Specifically, we do this when we see the API key in the "BuildStarted" event.
	return authutil.AuthContextWithError(ctx, nil)
}

func ClaimsFromContext(ctx context.Context) (*Claims, error) {
	// If the context already contains trusted Claims, return them directly
	// instead of re-parsing the JWT (which is expensive).
	if claims, ok := ctx.Value(contextClaimsKey).(*Claims); ok && claims != nil {
		return claims, nil
	}

	// If context already contains a JWT, just verify it and return the claims.
	if tokenString, ok := ctx.Value(authutil.ContextTokenStringKey).(string); ok && tokenString != "" {
		claims, err := ParseClaims(tokenString)
		if err != nil {
			return nil, err
		}
		return claims, nil
	}

	// If there's no error or we have an assertion failure; just return a
	// user not found error.
	err, ok := authutil.AuthErrorFromContext(ctx)
	if !ok || err == nil {
		return nil, authutil.AnonymousUserError(authutil.UserNotFoundMsg)
	}

	// if there was an error set on the context, and it was an
	// Unauthenticated or PermissionDeniedError, then the FE can handle it,
	// so pass it through. This includes anonymous user errors.
	if status.IsUnauthenticatedError(err) || status.IsPermissionDeniedError(err) {
		return nil, err
	}

	// All other types of errors will be converted into Unauthenticated
	// errors.
	// WARNING: app/auth/auth_service.ts depends on this status being UNAUTHENTICATED.
	return nil, status.UnauthenticatedErrorf("%s: %s", authutil.UserNotFoundMsg, err.Error())
}

// ClaimsCache helps reduce CPU overhead due to JWT parsing by caching parsed
// and verified JWT claims.
//
// The JWTs used with this cache should have Expiration times rounded down to
// the nearest minute, so that their cache key doesn't change as often and can
// therefore be cached for longer.
type ClaimsCache struct {
	ttl time.Duration

	mu  sync.Mutex
	lru interfaces.LRU[*Claims]
}

// Returns a ClaimsCache if the claims cache is enabled, or nil otherwise, or
// an error if there's an error constructing the cache.
//
// Note: this function can return (nil, nil)!
func NewClaimsCache() (*ClaimsCache, error) {
	if *claimsCacheTTL <= 0 {
		return nil, nil
	}

	config := &lru.Config[*Claims]{
		MaxSize: claimsCacheSize,
		SizeFn:  func(v *Claims) int64 { return 1 },
	}
	lru, err := lru.NewLRU[*Claims](config)
	if err != nil {
		return nil, err
	}
	return &ClaimsCache{ttl: *claimsCacheTTL, lru: lru}, nil
}

func (c *ClaimsCache) Get(token string) (*Claims, error) {
	c.mu.Lock()
	v, ok := c.lru.Get(token)
	c.mu.Unlock()

	if ok {
		if claims := v; claims.ExpiresAt > time.Now().Unix() {
			return claims, nil
		}
	}

	claims, err := ParseClaims(token)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.lru.Add(token, claims)
	c.mu.Unlock()

	return claims, nil
}
