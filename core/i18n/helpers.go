package i18n

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// maxAcceptLanguageLength prevents DoS attacks through oversized Accept-Language headers.
// RFC 7231 doesn't specify a limit, but 4KB is generous for legitimate headers while
// preventing memory exhaustion from malicious requests.
const maxAcceptLanguageLength = 4096

// languageTag represents a parsed language tag with quality value
type languageTag struct {
	tag     string
	quality float64
}

// ParseAcceptLanguage parses the Accept-Language header and returns the most
// applicable language from the available languages list.
// It supports quality values (q=0.9) and will match the highest quality
// available language. If no match is found, returns the first available language.
//
// Example header: "en-US,en;q=0.9,pl;q=0.8"
// Available: ["pl", "en", "de"]
// Returns: "en" (highest quality match)
func ParseAcceptLanguage(header string, available []string) string {
	if len(available) == 0 {
		return ""
	}

	if header == "" {
		return available[0]
	}

	// Parse the Accept-Language header
	tags := parseLanguageTags(header)

	// Find the best match from available languages
	var bestMatch string
	var bestQuality float64 = -1
	var bestIsExact bool

	// Iterate through server-preferred language order to respect server priorities
	// when client quality values are equal (RFC 7231 section 5.3.1)
	for _, avail := range available {
		availNorm := normalizeLanguageTag(avail)

		for _, tag := range tags {
			// Check for exact match
			if tag.tag == availNorm {
				// For exact matches: take if quality is better, or same quality but not yet exact
				if tag.quality > bestQuality || (tag.quality == bestQuality && !bestIsExact) {
					bestMatch = avail
					bestQuality = tag.quality
					bestIsExact = true
				}
				break // Found best possible match for this available language
			}

			// Check for partial match (e.g., "en" matches "en-US")
			if matchesLanguage(tag.tag, avail) {
				// Prioritize exact matches over partial matches, but accept better quality partial matches
				// when no exact match exists or when quality significantly improves
				if bestMatch == "" || (!bestIsExact && tag.quality > bestQuality) || (bestIsExact && tag.quality > bestQuality) {
					if !bestIsExact || tag.quality > bestQuality {
						bestMatch = avail
						bestQuality = tag.quality
						bestIsExact = false
					}
				}
				break
			}
		}
	}

	if bestMatch != "" {
		return bestMatch
	}

	// No match found, return first available
	return available[0]
}

// parseLanguageTags parses the Accept-Language header into language tags with quality values
func parseLanguageTags(header string) []languageTag {
	// Truncate oversized headers to prevent DoS
	if len(header) > maxAcceptLanguageLength {
		header = header[:maxAcceptLanguageLength]
	}

	var tags []languageTag

	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for quality value
		quality := 1.0
		langPart := part

		if idx := strings.Index(part, ";"); idx != -1 {
			langPart = strings.TrimSpace(part[:idx])
			qPart := strings.TrimSpace(part[idx+1:])

			// Parse q=value with validation
			if strings.HasPrefix(qPart, "q=") {
				if q, err := strconv.ParseFloat(qPart[2:], 64); err == nil && q >= 0 && q <= 1 {
					quality = q
				}
			}
		}

		if langPart != "" && langPart != "*" {
			tags = append(tags, languageTag{
				tag:     normalizeLanguageTag(langPart),
				quality: quality,
			})
		}
	}

	// Sort by quality score descending to respect user preferences
	slices.SortFunc(tags, func(a, b languageTag) int {
		return cmp.Compare(b.quality, a.quality) // Reversed for descending order
	})

	return tags
}

// normalizeLanguageTag normalizes a language tag to lowercase
func normalizeLanguageTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

// matchesLanguage checks if a requested language matches an available language
// Supports partial matching: "en" matches "en-us" and vice versa
func matchesLanguage(requested, available string) bool {
	requested = normalizeLanguageTag(requested)
	available = normalizeLanguageTag(available)

	// Exact match
	if requested == available {
		return true
	}

	// Check if one is a prefix of the other (en matches en-US)
	reqBase := strings.Split(requested, "-")[0]
	availBase := strings.Split(available, "-")[0]

	return reqBase == availBase
}

// ReplacePlaceholders replaces placeholders in the template string with values
// from the provided map. Placeholders use the format %{name}.
// If a placeholder is not found in the map, it remains unchanged.
//
// Example:
//
//	template: "Hello, %{name}! You have %{count} messages."
//	placeholders: M{"name": "John", "count": 5}
//	returns: "Hello, John! You have 5 messages."
func ReplacePlaceholders(template string, placeholders M) string {
	if len(placeholders) < 1 {
		return template
	}

	result := template
	for key, value := range placeholders {
		placeholder := fmt.Sprintf("%%{%s}", key)
		replacement := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, replacement)
	}

	return result
}
