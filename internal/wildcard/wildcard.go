// Package wildcard converts regular expressions to wildcard patterns for Surge.
package wildcard

import (
	"regexp/syntax"
	"strings"
)

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
