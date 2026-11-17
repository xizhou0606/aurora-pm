package httpapi

import (
	"database/sql"

	"gorm.io/gorm"
)

// Schedule validation locks the current WorkflowInstance before reading its
// runs. READ COMMITTED keeps those later reads aligned with the locked instance
// on MySQL without reversing the action path's NodeRun -> Instance lock order.
func workflowScheduleTransaction(db *gorm.DB, mutation func(*gorm.DB) error) error {
	return db.Transaction(mutation, workflowScheduleTransactionOptions())
}

func workflowScheduleTransactionOptions() *sql.TxOptions {
	return &sql.TxOptions{Isolation: sql.LevelReadCommitted}
}
