package router

import (
	"github.com/lute/api/handlers"
	"github.com/lute/api/middleware"
	"github.com/lute/api/repository"

	"github.com/gin-gonic/gin"
)

// SetupMachineRoutes sets up machine-specific routes
func SetupMachineRoutes(r *gin.RouterGroup, machineHandler *handlers.MachineHandler, userRepo *repository.UserRepository) {
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
			machines.DELETE("/:id", machineHandler.DeleteMachine)
		}
	}
}
