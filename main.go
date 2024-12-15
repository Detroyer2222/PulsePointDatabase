package main

import (
	"log"
	"net/http"

	"pulsepoint/internal/hooks"
	"pulsepoint/internal/tasks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/viper"
)

func main() {
	// Initialize the PocketBase application
	app := pocketbase.New()
	l := app.Logger().WithGroup("setup") // Create a logger for the setup process

	// Load configuration from the .env file using viper
	viper.SetConfigFile(".env")
	err := viper.ReadInConfig()
	if err != nil {
		l.Error("Error reading config file", "error", err)
		log.Fatalf("Error reading config file, %s", err)
	}
	l.Info("Config file loaded successfully")

	// Bind the serve function to define HTTP routes
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		l.Info("Setting up HTTP routes")

		// Register the route for updating commodities (with Superuser authentication)
		se.Router.POST("/api/pulsepoint/updateCommodities", func(e *core.RequestEvent) error {
			l.Info("Received request to update commodities")
			tasks.UpdateCommodities(app.App) // Call the UpdateCommodities task
			l.Info("Commodities updated successfully")
			return e.JSON(http.StatusOK, map[string]bool{"success": true})
			// Superuser authentication is required here when deploying
		}).Bind(apis.RequireSuperuserAuth())

		// Register the route for updating star systems (with Superuser authentication)
		se.Router.POST("/api/pulsepoint/updateStarSystems", func(e *core.RequestEvent) error {
			l.Info("Received request to update star systems")
			tasks.UpdateStarSystems(app.App) // Call the UpdateStarSystems task
			l.Info("Star systems updated successfully")
			return e.JSON(http.StatusOK, map[string]bool{"success": true})
			// Superuser authentication is required here when deploying
		}).Bind(apis.RequireSuperuserAuth())

		return se.Next()
	})

	// Add cron jobs to automatically update commodities and star systems
	l.Info("Scheduling cron jobs")
	app.Cron().MustAdd("updatingCommodities", "0 */6 * * *", func() {
		l.Info("Running cron job to update commodities")
		tasks.UpdateCommodities(app.App)
		l.Info("Commodities update completed by cron job")
	})
	app.Cron().MustAdd("updatingStarSystems", "0 12 1 */1 *", func() {
		l.Info("Running cron job to update star systems")
		tasks.UpdateStarSystems(app.App)
		l.Info("Star systems update completed by cron job")
	})

	// Hook for after a new outpost record is successfully created
	app.OnRecordAfterCreateSuccess("outposts").BindFunc(func(e *core.RecordEvent) error {
		l.Info("New outpost record created, triggering outpost commodities hook")
		hooks.CreateOutpostCommodities(e)
		l.Info("Outpost commodities created successfully")
		return e.Next()
	})

	// Hook for after an outpost_commodities record is updated
	app.OnRecordUpdateExecute("outpost_commodities").BindFunc(func(e *core.RecordEvent) error {
		l.Info("Outpost_commodities record updated, triggering commodity changes hook")
		hooks.CreateCommodityChanges(e)
		l.Info("Commodity changes created successfully")
		return e.Next()
	})

	// Start the application and handle errors
	l.Info("Starting PocketBase application")
	if err := app.Start(); err != nil {
		l.Error("Error starting PocketBase application", "error", err)
		log.Fatal(err)
	}
	l.Info("PocketBase application started successfully")
}
