package utils

import (
	"os/exec"
)

// IsInstalled checks if a program is installed on the system.
// It returns true if the program is found, and false otherwise.
// This function is intended for use on Unix-like systems (Linux, macOS).
func IsInstalled(programName string) bool {
	return exec.Command("which", programName).Run() == nil
}

// Uniq removes duplicate strings from a slice.
func Uniq(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// To be added after bumping to Go 1.18 or later
//func Uniq[T comparable](slice []T) []T {
//	seen := make(map[T]struct{})
//	result := []T{}
//
//	for _, v := range slice {
//		if _, ok := seen[v]; !ok {
//			seen[v] = struct{}{}
//			result = append(result, v)
//		}
//	}
//
//	return result
//}
