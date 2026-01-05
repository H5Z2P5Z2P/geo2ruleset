// Package wildcard converts regular expressions to wildcard patterns for Surge.
package wildcard

import (
	"regexp/syntax"
	"strings"
)

// IsDangerousRegex checks if a regex pattern would result in a dangerously broad wildcard match.
// Returns true if the regex contains constructs that lose precision when converted to wildcards.
func IsDangerousRegex(regex string) bool {
	// Remove leading and trailing slashes if present
	regex = strings.TrimPrefix(regex, "/")
	regex = strings.TrimSuffix(regex, "/")

	re, err := syntax.Parse(regex, syntax.Perl)
	if err != nil {
		return true // Can't parse = dangerous
	}

	// Check for dangerous constructs in AST
	if containsDangerousNode(re) {
		return true
	}

	// Also check if the converted result is too broad
	converted := convertNode(re)
	return isBroadPattern(converted)
}

// containsDangerousNode recursively checks for regex constructs that lose precision
func containsDangerousNode(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpCharClass:
		// Character classes like [a-z], [0-9] lose precision
		return true
	case syntax.OpAlternate:
		// Alternation like (a|b) loses precision
		return true
	case syntax.OpRepeat:
		// Repeat {n,m} loses the count constraint
		return true
	case syntax.OpStar, syntax.OpPlus:
		// Check if applying to something that already loses precision
		for _, sub := range re.Sub {
			if containsDangerousNode(sub) {
				return true
			}
		}
	case syntax.OpCapture, syntax.OpConcat:
		// Check all sub-expressions
		for _, sub := range re.Sub {
			if containsDangerousNode(sub) {
				return true
			}
		}
	}
	return false
}

// isBroadPattern checks if the converted pattern is too broad
func isBroadPattern(pattern string) bool {
	// Remove literal parts and check remaining wildcards
	wildcardOnly := true
	questionCount := 0

	for _, c := range pattern {
		switch c {
		case '*':
			// * is always broad
		case '?':
			questionCount++
		case '.':
			// Dots in domain names are fine
			wildcardOnly = false
		default:
			wildcardOnly = false
		}
	}

	// Pattern is too broad if:
	// 1. It consists only of wildcards (no literal chars except dots)
	// 2. It has many consecutive question marks (like ????.com)
	if wildcardOnly && len(pattern) > 0 {
		return true
	}

	// Check for patterns like "????.com" which match too many domains
	if questionCount >= 3 {
		return true
	}

	return false
}

// RegexToWildcard converts a regex pattern to a wildcard pattern.
// Returns the converted wildcard string.
func RegexToWildcard(regex string) string {
	// Remove leading and trailing slashes if present
	regex = strings.TrimPrefix(regex, "/")
	regex = strings.TrimSuffix(regex, "/")

	re, err := syntax.Parse(regex, syntax.Perl)
	if err != nil {
		return ""
	}

	return convertNode(re)
}

// convertNode recursively converts a regex AST node to wildcard pattern
func convertNode(re *syntax.Regexp) string {
	switch re.Op {
	case syntax.OpNoMatch:
		return ""
	case syntax.OpEmptyMatch:
		return ""
	case syntax.OpLiteral:
		// Return the literal characters
		return string(re.Rune)
	case syntax.OpCharClass:
		// Character class [abc] -> ?
		return "?"
	case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		// . -> ?
		return "?"
	case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText:
		// Anchors ^ $ -> empty
		return ""
	case syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		// Word boundaries -> empty
		return ""
	case syntax.OpCapture:
		// Capture group (abc) -> process sub
		var result strings.Builder
		for _, sub := range re.Sub {
			result.WriteString(convertNode(sub))
		}
		return result.String()
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest:
		// Quantifiers *, +, ? -> *
		return "*"
	case syntax.OpRepeat:
		// Repeat {n,m} -> *
		return "*"
	case syntax.OpConcat:
		// Concatenation abc -> abc
		var result strings.Builder
		for _, sub := range re.Sub {
			result.WriteString(convertNode(sub))
		}
		return result.String()
	case syntax.OpAlternate:
		// Alternation a|b -> *
		return "*"
	default:
		return "?"
	}
}
