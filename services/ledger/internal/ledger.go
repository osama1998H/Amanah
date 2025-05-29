package ledger

// Ledger defines methods for recording transaction entries.
type Ledger interface {
	// RecordTransaction logs a transaction entry in the ledger.
	RecordTransaction(tx interface{}) error
}
