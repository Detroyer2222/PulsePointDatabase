package hooks

import "github.com/pocketbase/pocketbase/core"

// CreateOutpostCommodities is a hook function that creates outpost commodity records whenever a new outpost is created.
// This function runs in a transaction to ensure atomicity. It first retrieves the necessary collections,
// then iterates over all commodities to create a corresponding outpost commodity for each commodity.
// Each created outpost commodity is linked to the newly created outpost and initialized with a quantity of 0.
//
// Parameters:
//   e (*core.RecordEvent): The event that triggered this hook, containing the newly created outpost record.
func CreateOutpostCommodities(e *core.RecordEvent) {
	l := e.App.Logger().WithGroup("createOutpostCommodities")

	// Start the transaction to ensure atomicity.
	l.Debug("Starting transaction to create outpost commodities", "outpost_id", e.Record.Id)

	e.App.RunInTransaction(func(txPb core.App) error {
		// Find the outpost_commodities collection
		outpostCommodityCollection, err := txPb.FindCollectionByNameOrId("outpost_commodities")
		if err != nil {
			l.Error("Error finding outpost_commodities collection", "error", err)
			return err
		}

		// Find all commodities to associate with the new outpost
		commodities, err := txPb.FindAllRecords("commodities", nil)
		if err != nil {
			l.Error("Error finding commodities", "error", err)
			return err
		}

		// Iterate over all commodities and create corresponding outpost commodities
		l.Debug("Found commodities, creating outpost commodities", "commodities_count", len(commodities))

		for _, commodity := range commodities {
			// Create a new outpost commodity for each commodity
			outpostCommodity := core.NewRecord(outpostCommodityCollection)
			outpostCommodity.Set("outpost", e.Record.Id)    // Link the outpost
			outpostCommodity.Set("commodity", commodity.Id) // Link the commodity
			outpostCommodity.Set("amount", 0)               // Initialize quantity as 0

			l.Debug("Creating outpost commodity", "outpost_id", e.Record.Id, "commodity_id", commodity.Id)

			// Save the new outpost commodity record
			if err := txPb.Save(outpostCommodity); err != nil {
				l.Error("Failed to save new outpost commodity", "error", err.Error(), "outpost_id", e.Record.Id, "commodity_id", commodity.Id)
				return err
			}
		}

		l.Info("Successfully created outpost commodities for new outpost", "outpost_id", e.Record.Id, "commodities_count", len(commodities))

		return nil
	})
}
