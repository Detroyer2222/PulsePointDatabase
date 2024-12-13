package main

import (
	"log"
	"os"

	"pulsepoint/internal/tasks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/viper"
)

func main() {
	app := pocketbase.New()

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// serves static files from the provided public dir (if exists)
		se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		return se.Next()
	})

	app.OnRecordAfterCreateSuccess("functionSecrets").BindFunc(func(e *core.RecordEvent) error {
		//tasks.UpdateCommodities(e)
		return e.Next()
	})

	viper.SetConfigFile(".env")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	app.Cron().MustAdd("updatingCommodities", "0 */6 * * *", func() { tasks.UpdateCommodities(app.App) })

	//app.OnServe().BindFunc(updateCommodities(commoditiesLogger))

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
