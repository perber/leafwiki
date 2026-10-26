package health

// HealthResponse is the body of GET /api/health, for both healthy (200) and
// degraded (503) results.
type HealthResponse struct {
	// Status is "ok" when every check passes, otherwise "degraded".
	Status string `json:"status"`
	// Checks maps each check name (sqlite, data_dir, search) to its result:
	// "ok" or "failed" for sqlite and data_dir; "ok", "indexing" or "failed" for search.
	Checks map[string]string `json:"checks"`
}
