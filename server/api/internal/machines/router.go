package machines

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
)

// SetupRoutes sets up machine-specific routes on the given router group.
func SetupRoutes(r *gin.RouterGroup, machineHandler *MachineHandler, userRepo *repos.UserRepository) {
	machines := r.Group("/machines")
	{
		// Public endpoint (no auth required)
		machines.GET("/public", machineHandler.ListPublicMachines)

		// Protected endpoints (require authentication)
		machines.Use(middleware.AuthMiddleware(userRepo))
		{
			machines.POST("", machineHandler.CreateMachine)
			machines.GET("", machineHandler.ListUserMachines)
			machines.GET("/:id", machineHandler.GetMachine)
			machines.PUT("/:id", machineHandler.UpdateMachine)
			machines.POST("/:id/re-enable", machineHandler.ReEnableMachine)
			machines.DELETE("/:id", machineHandler.DeleteMachine)
		}
	}
}
