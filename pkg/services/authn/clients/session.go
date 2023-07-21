package clients

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/network"
	"github.com/grafana/grafana/pkg/login/social"
	"github.com/grafana/grafana/pkg/services/auth"
	"github.com/grafana/grafana/pkg/services/authn"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/web"
	"golang.org/x/oauth2"
)

var _ authn.HookClient = new(Session)
var _ authn.ContextAwareClient = new(Session)

func ProvideSession(cfg *setting.Cfg, sessionService auth.UserTokenService, features *featuremgmt.FeatureManager, connector social.SocialConnector, httpClient *http.Client) *Session {
	return &Session{
		cfg:            cfg,
		features:       features,
		sessionService: sessionService,
		log:            log.New(authn.ClientSession),
		// Enale OAuth with session 
		connector:      connector,
		httpClient:     httpClient,
	}
}

type Session struct {
	cfg            *setting.Cfg
	features       *featuremgmt.FeatureManager
	sessionService auth.UserTokenService
	log            log.Logger

	// Enale OAuth with session 
	// Copied from OAuth.go
	connector  social.SocialConnector
	httpClient *http.Client
}

func (s *Session) Name() string {
	return authn.ClientSession
}

func (s *Session) Authenticate(ctx context.Context, r *authn.Request) (*authn.Identity, error) {
	unescapedCookie, err := r.HTTPRequest.Cookie(s.cfg.LoginCookieName)
	if err != nil {
		return nil, err
	}

	rawSessionToken, err := url.QueryUnescape(unescapedCookie.Value)
	if err != nil {
		return nil, err
	}

	token, sessionErr := s.sessionService.LookupToken(ctx, rawSessionToken)
	// if token found in session service,then use the origin `session` mechanism
	if sessionErr == nil {
		if s.features.IsEnabled(featuremgmt.FlagClientTokenRotation) {
			if token.NeedsRotation(time.Duration(s.cfg.TokenRotationIntervalMinutes) * time.Minute) {
				return nil, authn.ErrTokenNeedsRotation.Errorf("token needs to be rotated")
			}
		}
	
		return &authn.Identity{
			ID:           authn.NamespacedID(authn.NamespaceUser, token.UserId),
			SessionToken: token,
			ClientParams: authn.ClientParams{
				FetchSyncedUser: true,
				SyncPermissions: true,
			},
		}, nil
	}

	// if oauth is enabled,use OAuth
	if s.httpClient != nil && s.connector != nil {
		clientCtx := context.WithValue(ctx, oauth2.HTTPClient, s.httpClient)

		// read idtoken from http request
		token := &oauth2.Token{
			TokenType:   "Bearer",
			AccessToken: rawSessionToken,
		}

		userInfo, err := s.connector.UserInfo(s.connector.Client(clientCtx, token), token)
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

		if !s.connector.IsEmailAllowed(userInfo.Email) {
			return nil, errOAuthEmailNotAllowed.Errorf("provided email is not allowed")
		}

		// hardcode user role to GrafanaAdmin
		boolTrue := true
		return &authn.Identity{
			Login:          userInfo.Login,
			Name:           userInfo.Name,
			Email:          userInfo.Email,
			IsGrafanaAdmin: &boolTrue,
			AuthModule:     "oauth_generic_auth",
			AuthID:         userInfo.Id,
			Groups:         userInfo.Groups,
			OAuthToken:     token,

			ClientParams: authn.ClientParams{
				SyncUser:        true,
				SyncTeams:       true,
				FetchSyncedUser: true,
				SyncPermissions: true,
				AllowSignUp:     true,
			},
		}, nil
	}

	// return sessionErr if OAuth not enabled in seesion
	return nil,sessionErr
}

func (s *Session) Test(ctx context.Context, r *authn.Request) bool {
	if s.cfg.LoginCookieName == "" {
		return false
	}

	if _, err := r.HTTPRequest.Cookie(s.cfg.LoginCookieName); err != nil {
		return false
	}

	return true
}

func (s *Session) Priority() uint {
	return 60
}

func (s *Session) Hook(ctx context.Context, identity *authn.Identity, r *authn.Request) error {
	if identity.SessionToken == nil || s.features.IsEnabled(featuremgmt.FlagClientTokenRotation) {
		return nil
	}

	r.Resp.Before(func(w web.ResponseWriter) {
		if w.Written() || errors.Is(ctx.Err(), context.Canceled) {
			return
		}

		// FIXME (jguer): get real values
		addr := web.RemoteAddr(r.HTTPRequest)
		userAgent := r.HTTPRequest.UserAgent()

		// addr := reqContext.RemoteAddr()
		ip, err := network.GetIPFromAddress(addr)
		if err != nil {
			s.log.Debug("failed to get client IP address", "addr", addr, "err", err)
			ip = nil
		}
		rotated, newToken, err := s.sessionService.TryRotateToken(ctx, identity.SessionToken, ip, userAgent)
		if err != nil {
			s.log.Error("failed to rotate token", "error", err)
			return
		}

		if rotated {
			identity.SessionToken = newToken
			s.log.Debug("rotated session token", "user", identity.ID)

			authn.WriteSessionCookie(w, s.cfg, identity.SessionToken)
		}
	})

	return nil
}
