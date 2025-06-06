package handlers

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
