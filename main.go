package main

import (
	"log"
	"net/http"

	"pulsepoint/internal/hooks"
	"pulsepoint/internal/tasks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/viper"
)

func main() {
	app := pocketbase.New()

	viper.SetConfigFile(".env")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// register "POST /api/pulsepoint/updateCommodities" route (allowed only for authenticated users)
		se.Router.POST("/api/pulsepoint/updateCommodities", func(e *core.RequestEvent) error {
			tasks.UpdateCommodities(app.App)
			return e.JSON(http.StatusOK, map[string]bool{"success": true})
			// Add Superuser Auth here when deploying
		}).Bind( /*apis.RequireSuperuserAuth()*/ )

		// register "POST /api/pulsepoint/updateStarSystems" route (allowed only for authenticated users)
		se.Router.POST("/api/pulsepoint/updateStarSystems", func(e *core.RequestEvent) error {
			tasks.UpdateStarSystems(app.App)
			return e.JSON(http.StatusOK, map[string]bool{"success": true})
			// Add Superuser Auth here when deploying
		}).Bind( /*apis.RequireSuperuserAuth()*/ )

		return se.Next()
	})

	app.Cron().MustAdd("updatingCommodities", "0 */6 * * *", func() { tasks.UpdateCommodities(app.App) })
	app.Cron().MustAdd("updatingStarSystems", "0 12 1 */1 *", func() { tasks.UpdateStarSystems(app.App) })

	// fires only for "outposts"
	app.OnRecordAfterCreateSuccess("outposts").BindFunc(func(e *core.RecordEvent) error {
		hooks.CreateOutpostCommodities(e)
		return e.Next()
	})

	app.OnRecordUpdateExecute("outpost_commodities").BindFunc(func(e *core.RecordEvent) error {
		hooks.CreateCommodityChanges(e)
		return e.Next()
	})

	//TODO upload commit to github
	//TODO add uex_id to every collection and hide it
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
