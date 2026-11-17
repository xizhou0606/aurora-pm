package designreviewgorm

func MigrationRecords() []any {
	return []any{
		&Review{},
		&Asset{},
		&DifferenceLineage{},
		&Difference{},
		&LineageEvent{},
		&RecheckMatch{},
		&RecheckVerification{},
		&ResolutionEvent{},
		&AnalysisJob{},
		&DeepJob{},
		&ObjectCleanupJob{},
		&MobileCollector{},
		&MobilePairingCode{},
		&MobileCaptureDraft{},
	}
}
