package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/unicsmcr/hs_auth/config"
	"github.com/unicsmcr/hs_auth/environment"
	"github.com/unicsmcr/hs_auth/routers/api/models"
	"github.com/unicsmcr/hs_auth/services"
	"github.com/unicsmcr/hs_auth/utils/auth"
	authlevels "github.com/unicsmcr/hs_auth/utils/auth/common"
	"go.uber.org/zap"
)

const authHeaderName = "Authorization"

func jwtProvider(ctx *gin.Context) string {
	return ctx.GetHeader(authHeaderName)
}

func invalidJWTHandler(ctx *gin.Context) {
	models.SendAPIError(ctx, http.StatusUnauthorized, "you are not authorized to use this endpoint")
	ctx.Abort()
	return
}

// APIV1Router is the router for v1 of the API
type APIV1Router interface {
	models.Router
	GetUsers(*gin.Context)
	UpdateUser(*gin.Context)
	Login(*gin.Context)
	GetMe(*gin.Context)
	PutMe(*gin.Context)
	Register(*gin.Context)
	VerifyEmail(*gin.Context)
	GetTeams(*gin.Context)
	CreateTeam(*gin.Context)
	LeaveTeam(*gin.Context)
	JoinTeam(*gin.Context)
	GetTeamMembers(*gin.Context)
	GetTeammates(*gin.Context)
	GetPasswordResetEmail(*gin.Context)
	ResetPassword(*gin.Context)
}

type apiV1Router struct {
	models.BaseRouter
	logger       *zap.Logger
	cfg          *config.AppConfig
	userService  services.UserService
	emailService services.EmailService
	teamService  services.TeamService
	env          *environment.Env
}

// NewAPIV1Router creates a APIV1Router
func NewAPIV1Router(logger *zap.Logger, cfg *config.AppConfig, env *environment.Env, userService services.UserService, emailService services.EmailService, teamService services.TeamService) APIV1Router {
	return &apiV1Router{
		logger:       logger,
		cfg:          cfg,
		userService:  userService,
		emailService: emailService,
		teamService:  teamService,
		env:          env,
	}
}

// RegisterRoutes registers all of the API's (v1) routes to the given router group
func (r apiV1Router) RegisterRoutes(routerGroup *gin.RouterGroup) {
	routerGroup.GET("/", r.Heartbeat)

	isAtLeastApplicant := auth.AuthLevelVerifierFactory(authlevels.Applicant, jwtProvider, []byte(r.env.Get(environment.JWTSecret)), invalidJWTHandler)
	isAtLeastOrganiser := auth.AuthLevelVerifierFactory(authlevels.Organiser, jwtProvider, []byte(r.env.Get(environment.JWTSecret)), invalidJWTHandler)

	usersGroup := routerGroup.Group("/users")
	usersGroup.GET("/", isAtLeastOrganiser, r.GetUsers)
	// TODO: this endpoint cannot be accesible through PUT:/:id as PUT:/:id would conflict with PUT:/me
	//       Moving PUT:/me to a different endpoint would introduce breaking changes to the service's consumers
	// TODO: UpdateUser does not have input validation and is unsafe to use
	//usersGroup.PUT("/update/:id", isAtLeastOrganiser, r.UpdateUser)
	usersGroup.POST("/", r.Register)
	usersGroup.POST("/login", r.Login)
	usersGroup.POST("/email/verify", r.VerifyEmail)
	// TODO: GET:/me will conflict with /:id if such an endpoint is ever needed
	usersGroup.GET("/me", isAtLeastApplicant, r.GetMe)
	// TODO: this should be moved as PUT:/me conflicts with PUT:/:id
	usersGroup.PUT("/me", isAtLeastApplicant, r.PutMe)
	usersGroup.GET("/password/reset", r.GetPasswordResetEmail)
	usersGroup.PUT("/password/reset", r.ResetPassword)
	usersGroup.GET("/teammates", isAtLeastApplicant, r.GetTeammates)

	teamsGroup := routerGroup.Group("/teams")
	teamsGroup.GET("/", isAtLeastOrganiser, r.GetTeams)
	teamsGroup.POST("/", isAtLeastApplicant, r.CreateTeam)
	teamsGroup.GET("/:id/members", isAtLeastOrganiser, r.GetTeamMembers)
	teamsGroup.POST("/:id/join", isAtLeastApplicant, r.JoinTeam)
	teamsGroup.DELETE("/leave", isAtLeastApplicant, r.LeaveTeam)
}
