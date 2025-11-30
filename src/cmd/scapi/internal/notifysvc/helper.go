package notifysvc

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

func RenderTemplate(template *Template, data map[string]any) (subject, html, text string, err error) {
	if template == nil {
		return "", "", "", errors.New("notifysvc: template is nil")
	}

	subject = template.Subject
	html = template.HTML
	text = template.Text

	replacements := make(map[string]string, len(data))
	for k, v := range data {
		replacements[k] = fmt.Sprintf("%v", v)
	}

	subject = replacePlaceholders(subject, replacements)
	html = replacePlaceholders(html, replacements)
	text = replacePlaceholders(text, replacements)

	return subject, html, text, validatePlaceholders(subject, html, text, replacements)
}

func replacePlaceholders(content string, replacements map[string]string) string {
	result := content
	for key, value := range replacements {
		pattern := regexp.MustCompile(`{{\s*` + regexp.QuoteMeta(key) + `\s*}}`)
		result = pattern.ReplaceAllString(result, value)
	}
	return result
}

func validatePlaceholders(subject, html, text string, replacements map[string]string) error {
	combined := subject + html + text
	markers := extractPlaceholders(combined)
	for _, marker := range markers {
		if _, ok := replacements[marker]; !ok {
			return fmt.Errorf("notifysvc: placeholder %s not provided", marker)
		}
	}
	return nil
}

func extractPlaceholders(content string) []string {
	var markers []string
	start := 0

	for {
		open := strings.Index(content[start:], "{{")
		if open == -1 {
			break
		}
		start += open + 2

		close := strings.Index(content[start:], "}}")
		if close == -1 {
			break
		}

		key := strings.TrimSpace(content[start : start+close])
		if key != "" {
			markers = append(markers, key)
		}
		start += close + 2
	}

	return markers
}
