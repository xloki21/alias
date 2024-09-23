package urlparser

import "regexp"

// FindURLs finds all URLs in a text
func FindURLs(text string) ([]string, error) {
	urlPattern := `(?i)\b(?:https?://|www\.)\S+\.\S+\b`
	re := regexp.MustCompile(urlPattern)
	matches := re.FindAllString(text, -1)
	return matches, nil
}
