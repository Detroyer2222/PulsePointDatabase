package tasks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/viper"

	"github.com/pocketbase/pocketbase/core"
)

type Commodity struct {
	Name            string  `json:"name"`
	Code            string  `json:"code"`
	Type            string  `json:"kind"`
	PriceBuy        float64 `json:"price_buy"`
	PriceSell       float64 `json:"price_sell"`
	IsIllegal       int16   `json:"is_illegal"`
	IsAvailableLive int16   `json:"is_available_live"`
	IsTemporary     int16   `json:"is_temporary"`
	IsSellable      int16   `json:"is_sellable"`
}

type APIResponse struct {
	Data []Commodity `json:"data"`
}

// UpdateCommodities updates the commodities in the database
func UpdateCommodities(app core.App) {
	l := app.Logger().WithGroup("cronCommodities")

	l.Info("Updating commodities has started")
	fmt.Println("Updating commodities has started")

	// Loading the API URL and API Key from the database
	uexApiUrl, ok := viper.Get("UEX_API_URL").(string)
	if !ok {
		fmt.Errorf("Failed to get uexApiUrl")
	}

	uexApiKey, ok := viper.Get("UEX_API_KEY").(string)
	if !ok {
		fmt.Errorf("Failed to get uexApiUrl")
	}

	l.Debug("Uex Variables loaded",
		"url", uexApiUrl,
		"key", uexApiKey)
	fmt.Println("Uex Variables loaded")

	// Creating Request
	commodityUrl := fmt.Sprintf("%s/commodities", uexApiUrl)
	req, err := http.NewRequest("GET", commodityUrl, nil)
	if err != nil {
		l.Error("Failed to create request",
			"error", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uexApiKey))

	// Sending Request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		l.Error("Failed to send request",
			"error", err.Error())
		return
	}
	defer resp.Body.Close()

	// Reading Response
	if resp.StatusCode != http.StatusOK {
		l.Error("Failed to get commodities",
			"status", fmt.Sprintf("Status Code: %d", resp.StatusCode),
			"error", resp.Body)
		return
	}
	fmt.Println("Response received")

	var apiResponse APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		l.Error("Failed to decode response",
			"error", err.Error())
		return
	}

	// Saving to the database
	collection, err := app.FindCollectionByNameOrId("commodities")
	if err != nil {
		l.Error("Failed to get collection",
			"error", err.Error())
		fmt.Println("Failed to get collection")
		return
	}

	app.RunInTransaction(func(txPb core.App) error {
		fmt.Println("Starting transaction")
		for _, commodity := range apiResponse.Data {

			if commodity.IsAvailableLive == 0 || commodity.IsTemporary == 1 || commodity.IsSellable == 0 {
				l.Debug("Skipping commodity",
					"name", commodity.Name)
				fmt.Println("Skipping commodity")
				continue
			}

			if commodity.Type == "Temporary" && commodity.PriceSell == 0 {
				l.Debug("Skipping commodity",
					"name", commodity.Name)
				fmt.Println("Skipping commodity")
				continue
			}

			if ContainsIgnoreCase(commodity.Name, "year of the") {
				l.Debug("Skipping commodity",
					"name", commodity.Name)
				fmt.Println("Skipping commodity")
				continue
			}

			if ContainsIgnoreCase(commodity.Name, "ore") {
				commodity.Type = "Ore"
				fmt.Println("Changing Type to ore")
			}

			if ContainsIgnoreCase(commodity.Name, "raw") {
				commodity.Type = "Raw"
				fmt.Println("Changing Type to raw")
			}

			existingCommodity, err := txPb.FindFirstRecordByData("commodities", "code", commodity.Code)
			if err != nil {

				newCommodity := core.NewRecord(collection)
				newCommodity.Set("name", commodity.Name)
				newCommodity.Set("code", commodity.Code)
				newCommodity.Set("type", commodity.Type)
				newCommodity.Set("price_buy", commodity.PriceBuy)
				newCommodity.Set("price_sell", commodity.PriceSell)
				newCommodity.Set("is_illegal", ConvertToBool(commodity.IsIllegal))
				fmt.Println("Commodity not found, creating new")
				fmt.Println("Commodity: ", newCommodity)

				if err := txPb.Save(newCommodity); err != nil {
					l.Error("Failed to save new Commodity",
						"error", err.Error())
					fmt.Println("Failed to save new Commodity")
					return err
				}

			} else {
				fmt.Println("Commodity found, updating")

				existingCommodity.Set("type", commodity.Type)
				existingCommodity.Set("price_buy", commodity.PriceBuy)
				existingCommodity.Set("price_sell", commodity.PriceSell)
				existingCommodity.Set("is_illegal", commodity.IsIllegal)

				if err := txPb.Save(existingCommodity); err != nil {
					l.Error("Failed to save new Commodity",
						"error", err.Error())
					fmt.Println("Failed to update Commodity")
					return err
				}
			}
		}

		return nil
	})
}

func ContainsIgnoreCase(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

func ConvertToBool(val int16) bool {
	return val == 1
}

func UpdateStarSystems(app core.App) {
	//l := app.Logger().WithGroup("cronStarSystems")

}
