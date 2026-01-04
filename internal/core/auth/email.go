package auth

import "regexp"

// Simple email regex - validates most common email formats
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// IsValidEmail checks if email address is formatted correctly
func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}
