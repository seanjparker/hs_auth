package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/unicsmcr/hs_auth/testutils"

	"github.com/unicsmcr/hs_auth/environment"

	"github.com/stretchr/testify/assert"

	"github.com/gin-gonic/gin"

	mock_services "github.com/unicsmcr/hs_auth/mocks/services"

	"github.com/golang/mock/gomock"

	"go.uber.org/zap"
)

func Test_RegisterRoutes__should_register_required_routes(t *testing.T) {
	restoreVars := testutils.SetEnvVars(map[string]string{
		"JWT_SECRET": "verysecret",
	})
	restoreVars()
	env := environment.NewEnv(zap.NewNop())

	ctrl := gomock.NewController(t)
	mockUserService := mock_services.NewMockUserService(ctrl)

	mockUserService.EXPECT().GetUsers(gomock.Any()).AnyTimes()
	mockUserService.EXPECT().GetUserWithEmailAndPassword(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	router := NewAPIV1Router(zap.NewNop(), nil, mockUserService, env)

	w := httptest.NewRecorder()
	_, testServer := gin.CreateTestContext(w)

	router.RegisterRoutes(&testServer.RouterGroup)

	tests := []struct {
		route  string
		method string
	}{
		{
			route:  "/",
			method: http.MethodGet,
		},
		{
			route:  "/users",
			method: http.MethodGet,
		},
		{
			route:  "/users/verify",
			method: http.MethodGet,
		},
		{
			route:  "/users/login",
			method: http.MethodPost,
		},
		{
			route:  "/users/me",
			method: http.MethodGet,
		},
		{
			route:  "/users/me",
			method: http.MethodPut,
		},
	}

	for _, tt := range tests {
		t.Run(tt.route, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.route, nil)

			testServer.ServeHTTP(w, req)

			// making sure route is defined
			assert.NotEqual(t, http.StatusNotFound, w.Code)
		})
	}
}
