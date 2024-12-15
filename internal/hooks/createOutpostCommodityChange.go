package hooks

import "github.com/pocketbase/pocketbase/core"

// CreateCommodityChanges is a hook function that tracks and records changes in the commodity quantity
// for an outpost whenever a commodity record is updated. It compares the new commodity quantity with the previous
// quantity and logs the difference, saving this change as a new entry in the "commodity_changes" collection.
//
// Parameters:
//   e (*core.RecordEvent): The event that triggered this hook, containing the updated commodity record.
//
// Logs:
//   Detailed logs are created to capture:
//   - The start of the transaction.
//   - The old and new commodity records.
//   - The calculated quantity change.
func CreateCommodityChanges(e *core.RecordEvent) {
	l := e.App.Logger().WithGroup("createOutpostCommodityChange")

	// Start the transaction to ensure atomicity.
	l.Debug("Starting transaction to create commodity changes", "outpost_id", e.Record.Id)

	e.App.RunInTransaction(func(txPb core.App) error {
		// Retrieve the new and previous records to compare changes
		original := e.Record.Original().Clone()

		// Log both old and new commodity records for debugging purposes
		l.Debug("Old outpost commodity record", "old_record", original)
		l.Debug("New outpost commodity record", "new_record", e.Record)

		// Retrieve the commodity_changes collection to store the change record
		commodityChangesCollection, err := txPb.FindCollectionByNameOrId("commodity_changes")
		if err != nil {
			l.Error("Error finding commodity_changes collection", "error", err)
			return err
		}

		// Create a new record for the commodity change
		commodityChangeRecord := core.NewRecord(commodityChangesCollection)
		commodityChangeRecord.Set("outpost_commodity", e.Record.Id)
		commodityChangeRecord.Set("commodity", e.Record.Get("commodity"))

		// Calculate the change in quantity by comparing the new and previous values
		newAmount := e.Record.GetFloat("amount")
		previousAmount := original.GetFloat("amount")
		quantityChange := newAmount - previousAmount

		// Log the new and previous amounts for clarity
		l.Debug("Commodity quantity change", "new_amount", newAmount, "previous_amount", previousAmount, "quantity_change", quantityChange)

		// Set the quantity change in the new record
		commodityChangeRecord.Set("change_amount", quantityChange)

		// Save the commodity change record to the database
		if err := txPb.Save(commodityChangeRecord); err != nil {
			l.Error("Failed to save commodity change record", "error", err.Error())
			return err
		}

		l.Info("Successfully created commodity change record", "outpost_commodity_id", e.Record.Id, "commodity_id", e.Record.Get("commodity"))

		return nil
	})
}
