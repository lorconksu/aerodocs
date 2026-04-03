package model

type AuditHealth struct {
	FailureCount      int     `json:"failure_count"`
	LastFailureAt     *string `json:"last_failure_at"`
	LastFailureReason *string `json:"last_failure_reason"`
	Degraded          bool    `json:"degraded"`
	LastRecoveredAt   *string `json:"last_recovered_at"`
}

type AuditThresholds struct {
	LoginFailuresPerHour        int `json:"login_failures_per_hour"`
	RegistrationFailuresPerHour int `json:"registration_failures_per_hour"`
	PrivilegedActionsPerHour    int `json:"privileged_actions_per_hour"`
}

type AuditSettings struct {
	RetentionDays             int             `json:"retention_days"`
	ReviewReminderDays        int             `json:"review_reminder_days"`
	PasswordHistoryCount      int             `json:"password_history_count"`
	TemporaryPasswordTTLHours int             `json:"temporary_password_ttl_hours"`
	Thresholds                AuditThresholds `json:"thresholds"`
}

type AuditCatalogEntry struct {
	Action       string `json:"action"`
	Label        string `json:"label"`
	Category     string `json:"category"`
	Outcome      string `json:"outcome"`
	ActorType    string `json:"actor_type"`
	ResourceType string `json:"resource_type"`
}

type AuditCatalogResponse struct {
	Entries       []AuditCatalogEntry `json:"entries"`
	LastUpdatedAt string              `json:"last_updated_at"`
}

type AuditSavedFilter struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CreatedBy   string `json:"created_by"`
	FiltersJSON string `json:"filters_json"`
	UpdatedAt   string `json:"updated_at"`
	CreatedAt   string `json:"created_at"`
}

type AuditSavedFiltersResponse struct {
	Filters []AuditSavedFilter `json:"filters"`
}

type CreateAuditSavedFilterRequest struct {
	Name        string `json:"name"`
	FiltersJSON string `json:"filters_json"`
}

type AuditReview struct {
	ID          string  `json:"id"`
	ReviewerID  string  `json:"reviewer_id"`
	Reviewer    string  `json:"reviewer"`
	FiltersJSON string  `json:"filters_json"`
	Notes       string  `json:"notes"`
	From        *string `json:"from"`
	To          *string `json:"to"`
	CompletedAt string  `json:"completed_at"`
	CreatedAt   string  `json:"created_at"`
}

type AuditReviewsResponse struct {
	Reviews []AuditReview `json:"reviews"`
}

type CreateAuditReviewRequest struct {
	FiltersJSON string  `json:"filters_json"`
	Notes       string  `json:"notes"`
	From        *string `json:"from"`
	To          *string `json:"to"`
}

type AuditDetection struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type AuditDetectionsResponse struct {
	Detections []AuditDetection `json:"detections"`
}

type AuditManifest struct {
	GeneratedAt    string `json:"generated_at"`
	GeneratedBy    string `json:"generated_by"`
	RecordCount    int    `json:"record_count"`
	AppliedFilters string `json:"applied_filters"`
	FirstCreatedAt string `json:"first_created_at,omitempty"`
	LastCreatedAt  string `json:"last_created_at,omitempty"`
}

type AuditExportResponse struct {
	Manifest AuditManifest `json:"manifest"`
	Entries  []AuditEntry  `json:"entries"`
}

type AuditExportHistoryResponse struct {
	Entries []AuditEntry `json:"entries"`
}

type AuditFlag struct {
	ID          string  `json:"id"`
	EntryID     *string `json:"entry_id"`
	CreatedBy   string  `json:"created_by"`
	CreatedByID string  `json:"created_by_id"`
	FiltersJSON string  `json:"filters_json"`
	Note        string  `json:"note"`
	CreatedAt   string  `json:"created_at"`
}

type AuditFlagsResponse struct {
	Flags []AuditFlag `json:"flags"`
}

type CreateAuditFlagRequest struct {
	EntryID     *string `json:"entry_id"`
	FiltersJSON string  `json:"filters_json"`
	Note        string  `json:"note"`
}

type AuditRetentionRunResponse struct {
	DeletedCount int    `json:"deleted_count"`
	Cutoff       string `json:"cutoff"`
}
