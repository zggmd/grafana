package clients

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/login/social"
	"github.com/grafana/grafana/pkg/services/authn"
	"github.com/grafana/grafana/pkg/services/login"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/setting"
)

var _ authn.RedirectClient = new(OAuth2)

func ProvideOAuth2(
	name string, cfg *setting.Cfg, oauthCfg *social.OAuthInfo,
	connector social.SocialConnector, httpClient *http.Client,
) *OAuth2 {
	return &OAuth2{
		name + "2", fmt.Sprintf("oauth_%s", strings.TrimPrefix(name, "auth.client.")),
		log.New(name), cfg, oauthCfg, connector, httpClient,
	}
}

type OAuth2 struct {
	name       string
	moduleName string
	log        log.Logger
	cfg        *setting.Cfg
	oauthCfg   *social.OAuthInfo
	connector  social.SocialConnector
	httpClient *http.Client
}

func (c *OAuth2) Name() string {
	return c.name
}

func (c *OAuth2) Test(ctx context.Context, r *authn.Request) bool {
	authHeader := r.HTTPRequest.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}
	// Extract the token from the Authorization header
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return false
	}
	return true
}

func (c *OAuth2) Priority() uint {
	return 10
}

func (c *OAuth2) Authenticate(ctx context.Context, r *authn.Request) (*authn.Identity, error) {
	r.SetMeta(authn.MetaKeyAuthModule, c.moduleName)

	clientCtx := context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)

	authHeader := r.HTTPRequest.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("empty authorization header")
	}

	// Extract the token from the Authorization header
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, errors.New("invalid authorization header")
	}
	idToken := parts[1]

	// read idtoken from http request
	token := &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: idToken,
	}

	userInfo, err := c.connector.UserInfo(c.connector.Client(clientCtx, token), token)
	if err != nil {
		var sErr *social.Error
		if errors.As(err, &sErr) {
			return nil, fromSocialErr(sErr)
		}
		return nil, errOAuthUserInfo.Errorf("failed to get user info: %w", err)
	}

	if userInfo.Email == "" {
		return nil, errOAuthMissingRequiredEmail.Errorf("required attribute email was not provided")
	}

	if !c.connector.IsEmailAllowed(userInfo.Email) {
		return nil, errOAuthEmailNotAllowed.Errorf("provided email is not allowed")
	}

	orgRoles, isGrafanaAdmin, _ := getRoles(c.cfg, func() (org.RoleType, *bool, error) {
		if c.cfg.OAuthSkipOrgRoleUpdateSync {
			return "", nil, nil
		}
		return userInfo.Role, userInfo.IsGrafanaAdmin, nil
	})

	lookupParams := login.UserLookupParams{}
	if c.cfg.OAuthAllowInsecureEmailLookup {
		lookupParams.Email = &userInfo.Email
	}

	return &authn.Identity{
		Login:          userInfo.Login,
		Name:           userInfo.Name,
		Email:          userInfo.Email,
		IsGrafanaAdmin: isGrafanaAdmin,
		AuthModule:     c.moduleName,
		AuthID:         userInfo.Id,
		Groups:         userInfo.Groups,
		OAuthToken:     token,
		OrgRoles:       orgRoles,
		ClientParams: authn.ClientParams{
			SyncUser:        true,
			SyncTeams:       true,
			FetchSyncedUser: true,
			SyncPermissions: true,
			AllowSignUp:     c.connector.IsSignupAllowed(),
			// skip org role flag is checked and handled in the connector. For now we can skip the hook if no roles are passed
			SyncOrgRoles: len(orgRoles) > 0,
			LookUpParams: lookupParams,
		},
	}, nil
}

func (c *OAuth2) RedirectURL(ctx context.Context, r *authn.Request) (*authn.Redirect, error) {
	var opts []oauth2.AuthCodeOption

	if c.oauthCfg.HostedDomain != "" {
		opts = append(opts, oauth2.SetAuthURLParam(hostedDomainParamName, c.oauthCfg.HostedDomain))
	}

	var plainPKCE string
	if c.oauthCfg.UsePKCE {
		pkce, hashedPKCE, err := genPKCECode()
		if err != nil {
			return nil, errOAuthGenPKCE.Errorf("failed to generate pkce: %w", err)
		}

		plainPKCE = pkce
		opts = append(opts,
			oauth2.SetAuthURLParam(codeChallengeParamName, hashedPKCE),
			oauth2.SetAuthURLParam(codeChallengeMethodParamName, codeChallengeMethod),
		)
	}

	state, hashedSate, err := genOAuthState(c.cfg.SecretKey, c.oauthCfg.ClientSecret)
	if err != nil {
		return nil, errOAuthGenState.Errorf("failed to generate state: %w", err)
	}

	return &authn.Redirect{
		URL: c.connector.AuthCodeURL(state, opts...),
		Extra: map[string]string{
			authn.KeyOAuthState: hashedSate,
			authn.KeyOAuthPKCE:  plainPKCE,
		},
	}, nil
}
