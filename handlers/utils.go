package handlers

// contains проверяет наличие подстроки в строке
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
