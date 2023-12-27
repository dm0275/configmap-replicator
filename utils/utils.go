package utils

import "os"

func ListContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// SlicesOverlap Check if two slices have overlapping elements
func SlicesOverlap(slice1, slice2 []string) bool {
	// Create a map to store elements from slice1
	seen := make(map[string]bool)

	// Populate the map with elements from slice1
	for _, elem := range slice1 {
		seen[elem] = true
	}

	// Check if any element from slice2 exists in the map
	for _, elem := range slice2 {
		if seen[elem] {
			return true
		}
	}

	return false
}

func GetEnv(envVar, defaultVal string) string {
	environmentVar, exists := os.LookupEnv(envVar)
	if !exists {
		return defaultVal
	}
	return environmentVar
}
