package urlparser

import "regexp"

// ExtractURLsFromText extracts all URLs from text
func ExtractURLsFromText(text string) ([]string, error) {
	urlPattern := `(?i)\b(?:https?://|www\.)\S+\.\S+\b`
	re := regexp.MustCompile(urlPattern)
	matches := re.FindAllString(text, -1)
	return matches, nil
}
