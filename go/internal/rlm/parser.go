package rlm

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	finalTripleDouble = regexp.MustCompile(`(?s)FINAL\s*\(\s*"""(.*)"""`)
	finalTripleSingle = regexp.MustCompile(`(?s)FINAL\s*\(\s*'''(.*)'''`)
	finalDouble       = regexp.MustCompile(`(?s)FINAL\s*\(\s*"([^"]*)"`)
	finalSingle       = regexp.MustCompile(`(?s)FINAL\s*\(\s*'([^']*)'`)
	finalVar          = regexp.MustCompile(`FINAL_VAR\s*\(\s*(\w+)\s*\)`)
	finalAny          = regexp.MustCompile(`FINAL\(|FINAL_VAR\(`)
)

func IsFinal(response string) bool {
	return finalAny.MatchString(response)
}

func ParseResponse(response string, env map[string]interface{}) (string, bool) {
	answer, ok := extractFinal(response)
	if ok {
		return answer, true
	}

	return extractFinalVar(response, env)
}

func extractFinal(response string) (string, bool) {
	matchers := []*regexp.Regexp{finalTripleDouble, finalTripleSingle, finalDouble, finalSingle}
	for _, matcher := range matchers {
		match := matcher.FindStringSubmatch(response)
		if len(match) > 1 {
			return strings.TrimSpace(match[1]), true
		}
	}
	return "", false
}

func extractFinalVar(response string, env map[string]interface{}) (string, bool) {
	match := finalVar.FindStringSubmatch(response)
	if len(match) < 2 {
		return "", false
	}

	value, ok := env[match[1]]
	if !ok {
		return "", false
	}
	return fmt.Sprint(value), true
}
