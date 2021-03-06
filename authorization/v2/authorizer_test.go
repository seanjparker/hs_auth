package v2

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/unicsmcr/hs_auth/environment"
	mock_resources "github.com/unicsmcr/hs_auth/mocks/authorization/v2/resources"
	mock_utils "github.com/unicsmcr/hs_auth/mocks/utils"
	"github.com/unicsmcr/hs_auth/testutils"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type authorizerTestSetup struct {
	authorizer         Authorizer
	mockTimeProvider   *mock_utils.MockTimeProvider
	mockRouterResource *mock_resources.MockRouterResource
	testCtx            *gin.Context
	ctrl               *gomock.Controller
}

func setupAuthorizerTests(t *testing.T, jwtSecret string) authorizerTestSetup {
	restore := testutils.SetEnvVars(map[string]string{
		environment.JWTSecret: jwtSecret,
	})
	env := environment.NewEnv(zap.NewNop())
	restore()

	ctrl := gomock.NewController(t)
	mockTimeProvider := mock_utils.NewMockTimeProvider(ctrl)
	mockRouterResource := mock_resources.NewMockRouterResource(ctrl)

	w := httptest.NewRecorder()
	testCtx, _ := gin.CreateTestContext(w)
	testutils.AddRequestWithFormParamsToCtx(testCtx, http.MethodGet, nil)

	return authorizerTestSetup{
		authorizer:         NewAuthorizer(mockTimeProvider, env, zap.NewNop()),
		mockTimeProvider:   mockTimeProvider,
		mockRouterResource: mockRouterResource,
		testCtx:            testCtx,
		ctrl:               ctrl,
	}
}

func TestAuthorizer_CreateServiceToken(t *testing.T) {
	testOwner := "test_service"
	var testTTL int64 = 100
	testTimestamp := time.Now()
	testAllowedResources := []UniformResourceIdentifier{{path: "test"}}

	tests := []struct {
		name   string
		checks func(claims TokenClaims)
	}{
		{
			name: "should use correct Id",
			checks: func(claims TokenClaims) {
				assert.Equal(t, testOwner, claims.Id)
			},
		},
		{
			name: "should use correct IssuedAt",
			checks: func(claims TokenClaims) {
				assert.Equal(t, testTimestamp.Unix(), claims.IssuedAt)
			},
		},
		{
			name: "should use correct ExpiresAt",
			checks: func(claims TokenClaims) {
				assert.Equal(t, testTimestamp.Unix()+testTTL, claims.ExpiresAt)
			},
		},
		{
			name: "should use correct TokenType",
			checks: func(claims TokenClaims) {
				assert.Equal(t, service, claims.TokenType)
			},
		},
		{
			name: "should use correct AllowedResources",
			checks: func(claims TokenClaims) {
				assert.Equal(t, testAllowedResources, claims.AllowedResources)
			},
		},
	}

	jwtSecret := "test_secret"
	setup := setupAuthorizerTests(t, jwtSecret)
	setup.mockTimeProvider.EXPECT().Now().Return(testTimestamp).Times(1)

	token, err := setup.authorizer.CreateServiceToken(testOwner, testAllowedResources, testTimestamp.Unix()+testTTL)
	assert.NoError(t, err)

	claims := extractTokenClaims(t, token, jwtSecret)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checks(claims)
		})
	}
}

func TestAuthorizer_CreateUserToken(t *testing.T) {
	testUserId := primitive.NewObjectIDFromTimestamp(time.Now())
	var testTTL int64 = 100
	testTimestamp := time.Now()

	tests := []struct {
		name   string
		checks func(claims TokenClaims)
	}{
		{
			name: "should use correct Id",
			checks: func(claims TokenClaims) {
				assert.Equal(t, testUserId.Hex(), claims.Id)
			},
		},
		{
			name: "should use correct IssuedAt",
			checks: func(claims TokenClaims) {
				assert.Equal(t, testTimestamp.Unix(), claims.IssuedAt)
			},
		},
		{
			name: "should use correct ExpiresAt",
			checks: func(claims TokenClaims) {
				assert.Equal(t, testTimestamp.Unix()+testTTL, claims.ExpiresAt)
			},
		},
		{
			name: "should use correct TokenType",
			checks: func(claims TokenClaims) {
				assert.Equal(t, user, claims.TokenType)
			},
		},
	}

	jwtSecret := "test_secret"
	setup := setupAuthorizerTests(t, jwtSecret)
	setup.mockTimeProvider.EXPECT().Now().Return(testTimestamp).Times(1)

	token, err := setup.authorizer.CreateUserToken(testUserId, testTimestamp.Unix()+testTTL)
	assert.NoError(t, err)

	claims := extractTokenClaims(t, token, jwtSecret)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checks(claims)
		})
	}
}

func TestAuthorizer_GetAuthorizedResources_should_return_correct_uris_when_token_is_valid(t *testing.T) {
	jwtSecret := "test_secret"
	setup := setupAuthorizerTests(t, jwtSecret)
	token := createToken(t, "testuser", nil, int64(100), user, jwtSecret)
	uris := []UniformResourceIdentifier{{path: "test"}}

	returnedUris, err := setup.authorizer.GetAuthorizedResources(token, uris)
	assert.NoError(t, err)

	assert.Equal(t, uris, returnedUris)
}

func TestAuthorizer_GetAuthorizedResources_should_return_err(t *testing.T) {
	jwtSecret := "jwtSecret"

	tests := []struct {
		name      string
		token     string
		wantedErr error
	}{
		{
			name:      "when token is invalid",
			token:     "invalid token",
			wantedErr: ErrInvalidToken,
		},
		{
			name:      "when token type is invalid",
			token:     createToken(t, "user id", nil, int64(0), "unknown type", jwtSecret),
			wantedErr: ErrInvalidToken,
		},
		{
			name:      "when token is expired",
			token:     createToken(t, "user id", nil, int64(-5), user, jwtSecret),
			wantedErr: ErrInvalidToken,
		},
	}

	setup := setupAuthorizerTests(t, jwtSecret)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uris, err := setup.authorizer.GetAuthorizedResources(tt.token, nil)
			assert.Nil(t, uris)
			assert.Equal(t, tt.wantedErr, errors.Cause(err))
		})
	}
}

func Test_verifyTokenType(t *testing.T) {
	tests := []struct {
		tokenType TokenType
		wantErr   bool
	}{
		{
			tokenType: user,
		},
		{
			tokenType: service,
		},
		{
			tokenType: "unknown type",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.tokenType), func(t *testing.T) {
			assert.Equal(t, tt.wantErr, verifyTokenType(tt.tokenType) != nil)
		})
	}
}

func TestAuthorizer_WithAuthMiddleware_should_call_HandleUnauthorized(t *testing.T) {
	tests := []struct {
		name string
		prep func(*authorizerTestSetup)
	}{
		{
			name: "when token is empty",
			prep: func(setup *authorizerTestSetup) {
				setup.mockRouterResource.EXPECT().GetAuthToken(gomock.Any()).Return("", nil).Times(1)
			},
		},
		{
			name: "when GetAuthToken returns err",
			prep: func(setup *authorizerTestSetup) {
				setup.mockRouterResource.EXPECT().GetAuthToken(gomock.Any()).Return("123123", errors.New("test err")).Times(1)
			},
		},
		{
			name: "when GetAuthorizedResources returns err",
			prep: func(setup *authorizerTestSetup) {
				setup.mockRouterResource.EXPECT().GetAuthToken(gomock.Any()).Return("invalid_token", nil).Times(1)
				setup.mockRouterResource.EXPECT().GetResourcePath().Return("resource").Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := setupAuthorizerTests(t, "")
			defer setup.ctrl.Finish()
			mockHandler := func(*gin.Context) {}
			tt.prep(&setup)

			setup.mockRouterResource.EXPECT().HandleUnauthorized(gomock.Any()).Times(1)

			wrappedHandler := setup.authorizer.WithAuthMiddleware(setup.mockRouterResource, mockHandler)

			wrappedHandler(setup.testCtx)
		})
	}

}

func TestAuthorizer_WithAuthMiddleware_should_call_handler_when_request_is_authorized(t *testing.T) {
	setup := setupAuthorizerTests(t, "")
	defer setup.ctrl.Finish()
	mockHandlerCalled := false
	mockHandler := func(*gin.Context) { mockHandlerCalled = true }

	token := createToken(t, "test_token", nil, int64(10000), service, "")
	setup.mockRouterResource.EXPECT().GetAuthToken(gomock.Any()).Return(token, nil).Times(1)
	setup.mockRouterResource.EXPECT().GetResourcePath().Return("resource").Times(1)

	wrappedHandler := setup.authorizer.WithAuthMiddleware(setup.mockRouterResource, mockHandler)

	wrappedHandler(setup.testCtx)

	assert.True(t, mockHandlerCalled)
}

func createToken(t *testing.T, id string, allowedResources []UniformResourceIdentifier, timeToLive int64, tokenType TokenType, jwtSecret string) string {
	token := jwt.NewWithClaims(jwtSigningMethod, TokenClaims{
		StandardClaims: jwt.StandardClaims{
			Id:        id,
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Unix() + timeToLive,
		},
		TokenType:        tokenType,
		AllowedResources: allowedResources,
	})

	tokenStr, err := token.SignedString([]byte(jwtSecret))
	assert.NoError(t, err)

	return tokenStr
}

func extractTokenClaims(t *testing.T, token string, jwtSecret string) TokenClaims {
	var claims TokenClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	assert.NoError(t, err)

	return claims
}
