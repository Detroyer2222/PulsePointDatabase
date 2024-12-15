package tasks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/viper"

	"github.com/pocketbase/pocketbase/core"
)

type CommodityResponse struct {
	Data []Commodity `json:"data"`
}

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

// UpdateCommodities is a function that fetches commodity data from an external API
// and updates the local database accordingly. The function retrieves API credentials
// from the configuration, makes an HTTP request to fetch the commodity data, and processes
// the received data to update or insert commodities into the database.
// It also ensures only valid and non-temporary commodities are processed and saved.
func UpdateCommodities(app core.App) {
	l := app.Logger().WithGroup("cronCommodities")

	// Log the start of the commodity update process
	l.Info("Updating commodities has started")
	fmt.Println("Updating commodities has started")

	// Loading the API URL and API Key from the database or config
	uexApiUrl, ok := viper.Get("UEX_API_URL").(string)
	if !ok {
		l.Error("Failed to get UEX API URL from config")
		return
	}

	uexApiKey, ok := viper.Get("UEX_API_KEY").(string)
	if !ok {
		l.Error("Failed to get UEX API Key from config")
		return
	}

	// Log the loaded API variables (for debugging purposes)
	l.Debug("UEX API variables loaded", "url", uexApiUrl, "key", uexApiKey)

	// Construct the URL for fetching commodity data
	commodityUrl := fmt.Sprintf("%s/commodities", uexApiUrl)
	req, err := http.NewRequest("GET", commodityUrl, nil)
	if err != nil {
		l.Error("Failed to create HTTP request", "error", err.Error())
		return
	}

	// Set the necessary headers for the request
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uexApiKey))

	// Send the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		l.Error("Failed to send HTTP request", "error", err.Error())
		return
	}
	defer resp.Body.Close()

	// Log the response status code
	l.Debug("Received response from UEX API", "status_code", resp.StatusCode)

	// Handle cases where the response status code is not OK
	if resp.StatusCode != http.StatusOK {
		l.Error("Failed to get commodities", "status", fmt.Sprintf("Status Code: %d", resp.StatusCode))
		return
	}

	// Decode the API response into a CommodityResponse struct
	var apiResponse CommodityResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		l.Error("Failed to decode API response", "error", err.Error())
		return
	}

	// Log the successful response parsing
	l.Debug("Successfully decoded API response", "commodities_count", len(apiResponse.Data))

	// Access the commodities collection from the database
	collection, err := app.FindCollectionByNameOrId("commodities")
	if err != nil {
		l.Error("Failed to get commodities collection", "error", err.Error())
		return
	}

	// Begin a transaction to update or insert commodities
	app.RunInTransaction(func(txPb core.App) error {
		for _, commodity := range apiResponse.Data {

			// Skip invalid or temporary commodities
			if commodity.IsAvailableLive == 0 || commodity.IsTemporary == 1 || commodity.IsSellable == 0 {
				l.Debug("Skipping commodity due to invalid status", "name", commodity.Name)
				continue
			}

			// Skip commodities with a price of 0 and type "Temporary"
			if commodity.Type == "Temporary" && commodity.PriceSell == 0 {
				l.Debug("Skipping commodity with price 0 and type 'Temporary'", "name", commodity.Name)
				continue
			}

			// Skip commodities that contain "year of the"
			if ContainsIgnoreCase(commodity.Name, "year of the") {
				l.Debug("Skipping commodity containing 'year of the'", "name", commodity.Name)
				continue
			}

			// Update commodity type based on its name
			if ContainsIgnoreCase(commodity.Name, "ore") {
				commodity.Type = "Ore"
			}

			if ContainsIgnoreCase(commodity.Name, "raw") {
				commodity.Type = "Raw"
			}

			// Check if the commodity already exists in the database
			existingCommodity, err := txPb.FindFirstRecordByData("commodities", "code", commodity.Code)
			if err != nil {
				// Create a new commodity record if it doesn't exist
				l.Debug("Commodity does not exist, creating new record", "name", commodity.Name)

				newCommodity := core.NewRecord(collection)
				newCommodity.Set("name", commodity.Name)
				newCommodity.Set("code", commodity.Code)
				newCommodity.Set("type", commodity.Type)
				newCommodity.Set("price_buy", commodity.PriceBuy)
				newCommodity.Set("price_sell", commodity.PriceSell)
				newCommodity.Set("is_illegal", ConvertToBool(commodity.IsIllegal))

				// Save the new commodity record to the database
				if err := txPb.Save(newCommodity); err != nil {
					l.Error("Failed to save new commodity", "name", commodity.Name, "error", err.Error())
					return err
				}

			} else {
				// Update existing commodity record
				l.Debug("Updating existing commodity", "name", commodity.Name)

				existingCommodity.Set("type", commodity.Type)
				existingCommodity.Set("price_buy", commodity.PriceBuy)
				existingCommodity.Set("price_sell", commodity.PriceSell)
				existingCommodity.Set("is_illegal", commodity.IsIllegal)

				// Save the updated commodity record to the database
				if err := txPb.Save(existingCommodity); err != nil {
					l.Error("Failed to update commodity", "name", commodity.Name, "error", err.Error())
					return err
				}
			}
		}

		return nil
	})

	// Log the completion of the commodity update process
	l.Info("Commodity update process has completed")
}

// ContainsIgnoreCase checks if a substring (substr) is present within a string (str),
// ignoring case differences. It converts both strings to lowercase before performing the check.
//
// Parameters:
//
//	str (string): The string in which to search for the substring.
//	substr (string): The substring to search for within the string.
//
// Returns:
//
//	bool: Returns true if the substring is found within the string, ignoring case; otherwise, false.
func ContainsIgnoreCase(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

// ConvertToBool converts an integer value (typically representing a boolean in some systems)
// to a Go boolean. It returns true if the value is 1, and false if the value is any other integer.
//
// Parameters:
//
//	val (int16): The integer value to be converted to a boolean.
//
// Returns:
//
//	bool: Returns true if the value is 1, false otherwise.
func ConvertToBool(val int16) bool {
	return val == 1
}

type StarSystemResponse struct {
	Data []StarSystem `json:"data"`
}

type StarSystem struct {
	UexID        int16  `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	Jurisdiction string `json:"jurisdiction"`
	Faction      string `json:"faction"`
	IsAvailable  int16  `json:"is_available"`
	IsVisible    int16  `json:"is_visible"`
}

type PlanetResponse struct {
	Data []Planet `json:"data"`
}

type Planet struct {
	UexID        int16  `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	Jurisdiction string `json:"jurisdiction"`
	Faction      string `json:"faction"`
}

type MoonResponse struct {
	Data []Moon `json:"data"`
}

type Moon struct {
	UexID        int16  `json:"id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	PlanetName   string `json:"planet_name"`
	Jurisdiction string `json:"jurisdiction"`
	Faction      string `json:"faction"`
}

type SpaceStationResponse struct {
	Data []SpaceStation `json:"data"`
}

type SpaceStation struct {
	UexID          int16  `json:"id"`
	StarSystemName string `json:"star_system_name"`
	PlanetName     string `json:"planet_name"`
	MoonName       string `json:"moon_name"`
	Name           string `json:"name"`
	Code           string `json:"code"`
	PadTypes       string `json:"pad_types"`
	Jurisdiction   string `json:"jurisdiction"`
	Faction        string `json:"faction"`
	HasTerminal    int16  `json:"has_trade_terminal"`
	HasRefinery    int16  `json:"has_refinery"`
	Orbit          string `json:"orbit_name"`
	IsLagrange     int16  `json:"is_lagrange"`
}

func UpdateStarSystems(app core.App) {
	l := app.Logger().WithGroup("cronStarSystems")

	l.Info("Updating star systems has started")

	uexApiUrl, ok := viper.Get("UEX_API_URL").(string)
	if !ok {
		l.Error("Failed to get uexApiUrl")
	}

	uexApiKey, ok := viper.Get("UEX_API_KEY").(string)
	if !ok {
		l.Error("Failed to get uexApiUrl")
	}

	l.Debug("Uex Variables loaded",
		"url", uexApiUrl,
		"key", uexApiKey)

	// Creating System Request
	starsystemUrl := fmt.Sprintf("%s/star_systems", uexApiUrl)
	req, err := http.NewRequest("GET", starsystemUrl, nil)
	if err != nil {
		l.Error("Failed to create request",
			"error", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uexApiKey))

	//Sending Request
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
		l.Error("Failed to get star systems",
			"status", fmt.Sprintf("Status Code: %d", resp.StatusCode),
			"error", resp.Body)
		return
	}

	var apiResponse StarSystemResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		l.Error("Failed to decode Star System response",
			"error", err.Error())
		return
	}

	// Filtering out the data
	var relevantSystems []StarSystem
	for _, system := range apiResponse.Data {
		if system.IsAvailable == 1 && system.IsVisible == 1 {
			relevantSystems = append(relevantSystems, system)
		}
	}

	// Saving to the database
	starSystemCollection, err := app.FindCollectionByNameOrId("star_systems")
	if err != nil {
		l.Error("Failed to get collection",
			"error", err.Error())
		return
	}

	app.RunInTransaction(func(txPb core.App) error {
		l.Debug("Starting transaction")

		for _, system := range relevantSystems {
			l.Debug("System",
				"name", system.Name,
				"code", system.Code,
				"uex_id", system.UexID)

			existingSystem, err := txPb.FindFirstRecordByData("star_systems", "code", system.Code)
			if err != nil {
				l.Debug("System not found, creating new")

				newSystem := core.NewRecord(starSystemCollection)
				newSystem.Set("name", system.Name)
				newSystem.Set("code", system.Code)
				newSystem.Set("jurisdiction", system.Jurisdiction)
				newSystem.Set("faction", system.Faction)

				l.Debug("System",
					"id", newSystem)

				if err := txPb.Save(newSystem); err != nil {
					l.Error("Failed to save new System",
						"error", err.Error())
					return err
				}

			} else {
				l.Debug("System found, updating")

				existingSystem.Set("jurisdiction", system.Jurisdiction)
				existingSystem.Set("faction", system.Faction)

				if err := txPb.Save(existingSystem); err != nil {
					l.Error("Failed to save new System",
						"error", err.Error())
					fmt.Println("Failed to update System")
					return err
				}
			}
		}
		return nil
	})

	// Planets
	for _, system := range relevantSystems {
		// Create planet Request
		planetsUrl := fmt.Sprintf("%s/planets?id_star_system=%d", uexApiUrl, system.UexID)
		planetReq, planetErr := http.NewRequest("GET", planetsUrl, nil)
		if planetErr != nil {
			l.Error("Failed to create request",
				"error", planetErr.Error())
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uexApiKey))

		//Sending Request
		planetClient := &http.Client{}
		planetResp, planetErr := planetClient.Do(planetReq)
		if planetErr != nil {
			l.Error("Failed to send request",
				"error", planetErr.Error())
			return
		}
		defer planetResp.Body.Close()

		// Reading Response
		if planetResp.StatusCode != http.StatusOK {
			l.Error("Failed to get planets",
				"status", planetResp.Status)
			return
		}

		var apiResponse PlanetResponse
		err = json.NewDecoder(planetResp.Body).Decode(&apiResponse)
		if err != nil {
			l.Error("Failed to decode Star System response",
				"error", err.Error())
			return
		}

		// Updating Database
		planetsCollection, err := app.FindCollectionByNameOrId("planets")
		if err != nil {
			l.Error("Failed to get collection",
				"error", err.Error())
			return
		}

		app.RunInTransaction(func(txPb core.App) error {
			l.Debug("Starting Transaction")

			for _, planet := range apiResponse.Data {

				existingPlanet, err := txPb.FindFirstRecordByData("planets", "code", planet.Code)
				if err != nil {
					l.Debug("Planet not found, creating new")

					newPlanet := core.NewRecord(planetsCollection)
					newPlanet.Set("name", planet.Name)
					newPlanet.Set("code", planet.Code)
					newPlanet.Set("jurisdiction", planet.Jurisdiction)
					newPlanet.Set("faction", planet.Faction)

					if err := txPb.Save(newPlanet); err != nil {
						l.Error("Failed to save new planet",
							"error", err.Error())
						return err
					}
				} else {
					l.Debug("Planet found, updating")

					existingPlanet.Set("name", planet.Name)
					existingPlanet.Set("code", planet.Code)
					existingPlanet.Set("jurisdiction", planet.Jurisdiction)
					existingPlanet.Set("faction", planet.Faction)

					if err := txPb.Save(existingPlanet); err != nil {
						l.Error("Failed to update planet",
							"error", err.Error())
						return err
					}
				}
			}
			return nil
		})

		// Moons
		for _, system := range relevantSystems {
			// Create Moon Request
			moonsUrl := fmt.Sprintf("%s/moons?id_star_system=%d", uexApiUrl, system.UexID)
			moonReq, moonErr := http.NewRequest("GET", moonsUrl, nil)
			if moonErr != nil {
				l.Error("Failed to create request",
					"error", moonErr.Error())
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uexApiKey))

			//Sending Request
			moonClient := &http.Client{}
			moonResp, moonErr := moonClient.Do(moonReq)
			if moonErr != nil {
				l.Error("Failed to send request",
					"error", moonErr.Error())
				return
			}
			defer moonResp.Body.Close()

			// Reading Response
			if moonResp.StatusCode != http.StatusOK {
				l.Error("Failed to get moons",
					"status", moonResp.Status)
				return
			}

			var apiResponse MoonResponse
			err = json.NewDecoder(moonResp.Body).Decode(&apiResponse)
			if err != nil {
				l.Error("Failed to decode Star System response",
					"error", err.Error())
				return
			}

			// Updating Database
			moonsCollection, err := app.FindCollectionByNameOrId("moons")
			if err != nil {
				l.Error("Failed to get collection",
					"error", err.Error())
				return
			}

			app.RunInTransaction(func(txPb core.App) error {
				l.Debug("Starting Transaction")

				for _, moon := range apiResponse.Data {

					existingPlanet, err := txPb.FindFirstRecordByData("planets", "name", moon.PlanetName)
					if err != nil {
						l.Debug("Planet of Moon not found")
						continue
					}

					existingMoon, err := txPb.FindFirstRecordByData("moons", "code", moon.Code)
					if err != nil {
						l.Debug("Moon not found, creating new")

						newMoon := core.NewRecord(moonsCollection)
						newMoon.Set("name", moon.Name)
						newMoon.Set("code", moon.Code)
						newMoon.Set("planet", existingPlanet.Id)
						newMoon.Set("jurisdiction", moon.Jurisdiction)
						newMoon.Set("faction", moon.Faction)

						if err := txPb.Save(newMoon); err != nil {
							l.Error("Failed to save new moon",
								"error", err.Error())
							return err
						}
					} else {
						l.Debug("Moon found, updating")

						existingMoon.Set("name", moon.Name)
						existingMoon.Set("code", moon.Code)
						existingMoon.Set("planet", existingPlanet.Id)
						existingMoon.Set("jurisdiction", moon.Jurisdiction)
						existingMoon.Set("faction", moon.Faction)

						if err := txPb.Save(existingMoon); err != nil {
							l.Error("Failed to update moon",
								"error", err.Error())
							return err
						}
					}
				}
				return nil
			})
		}

		// Space Stations
		for _, system := range relevantSystems {
			l.Info("Updating space stations")

			// Create Space Station Request
			spaceStationUrl := fmt.Sprintf("%s/space_stations?id_star_system=%d", uexApiUrl, system.UexID)
			spaceStationReq, spaceStationErr := http.NewRequest("GET", spaceStationUrl, nil)
			if spaceStationErr != nil {
				l.Error("Failed to create request",
					"error", spaceStationErr.Error())
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uexApiKey))

			//Sending Request
			spaceStationClient := &http.Client{}
			spaceStationResp, spaceStationErr := spaceStationClient.Do(spaceStationReq)
			if spaceStationErr != nil {
				l.Error("Failed to send request",
					"error", spaceStationErr.Error())
				return
			}
			defer spaceStationResp.Body.Close()

			// Reading Response
			if spaceStationResp.StatusCode != http.StatusOK {
				l.Error("Failed to get space stations",
					"status", spaceStationResp.Status)
				return
			}

			var apiResponse SpaceStationResponse
			err = json.NewDecoder(spaceStationResp.Body).Decode(&apiResponse)
			if err != nil {
				l.Error("Failed to decode Star System response",
					"error", err.Error())
				return
			}

			// Updating Database
			spaceStationsCollection, err := app.FindCollectionByNameOrId("space_stations")
			if err != nil {
				l.Error("Failed to get collection",
					"error", err.Error())
				return
			}

			app.RunInTransaction(func(txPb core.App) error {
				l.Debug("Starting Transaction")

				for _, spaceStation := range apiResponse.Data {

					existingStarSystem, err := txPb.FindFirstRecordByData("star_systems", "name", spaceStation.StarSystemName)
					if err != nil {
						l.Error("Failed to get star system",
							"error", err.Error())
						return err
					}

					var existingPlanet *core.Record
					var existingMoon *core.Record

					if spaceStation.PlanetName != "" {
						p, err := txPb.FindFirstRecordByData("planets", "name", spaceStation.PlanetName)
						if err != nil {
							l.Info("Space Station is not orbiting a planet")
						} else {
							existingPlanet = p
						}

						if spaceStation.MoonName != "" {
							m, err := txPb.FindFirstRecordByData("moons", "name", spaceStation.MoonName)
							if err != nil {
								l.Info("Space Station is not orbiting a moon")
							} else {
								existingMoon = m
							}
						}
					}

					existingSpaceStation, err := txPb.FindFirstRecordByData("space_stations", "name", spaceStation.Name)
					if err != nil {
						l.Debug("Space Station not found, creating new")

						newSpaceStation := core.NewRecord(spaceStationsCollection)
						newSpaceStation.Set("name", spaceStation.Name)
						newSpaceStation.Set("pad_types", spaceStation.PadTypes)
						newSpaceStation.Set("jurisdiction", spaceStation.Jurisdiction)
						newSpaceStation.Set("faction", spaceStation.Faction)
						newSpaceStation.Set("has_trade_terminal", ConvertToBool(spaceStation.HasTerminal))
						newSpaceStation.Set("has_refinery", ConvertToBool(spaceStation.HasRefinery))
						newSpaceStation.Set("star_system", existingStarSystem.Id)
						if existingPlanet != nil {
							newSpaceStation.Set("planet", existingPlanet.Id)
						}
						if existingMoon != nil {
							newSpaceStation.Set("moon", existingMoon.Id)
						}
						newSpaceStation.Set("orbit", spaceStation.Orbit)
						newSpaceStation.Set("is_lagrange", ConvertToBool(spaceStation.IsLagrange))

						if err := txPb.Save(newSpaceStation); err != nil {
							l.Error("Failed to save new space station",
								"error", err.Error())
							return err
						}

					} else {
						l.Debug("Space Station found, updating")

						existingSpaceStation.Set("name", spaceStation.Name)
						existingSpaceStation.Set("pad_types", spaceStation.PadTypes)
						existingSpaceStation.Set("jurisdiction", spaceStation.Jurisdiction)
						existingSpaceStation.Set("faction", spaceStation.Faction)
						existingSpaceStation.Set("has_trade_terminal", ConvertToBool(spaceStation.HasTerminal))
						existingSpaceStation.Set("has_refinery", ConvertToBool(spaceStation.HasRefinery))
						existingSpaceStation.Set("star_system", existingStarSystem.Id)
						if existingPlanet != nil {
							existingSpaceStation.Set("planet", existingPlanet.Id)
						}
						if existingMoon != nil {
							existingSpaceStation.Set("moon", existingMoon.Id)
						}
						existingSpaceStation.Set("orbit", spaceStation.Orbit)
						existingSpaceStation.Set("is_lagrange", ConvertToBool(spaceStation.IsLagrange))

						if err := txPb.Save(existingSpaceStation); err != nil {
							l.Error("Failed to update space station",
								"error", err.Error())
							return err
						}
					}
				}
				return nil
			})
		}
	}
}
