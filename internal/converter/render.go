// Package converter handles the conversion of v2fly domain list format to ruleset formats.
package converter

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/xxxbrian/surge-geosite/internal/wildcard"
)

// skipPattern matches patterns that result in only wildcards
var skipPattern = regexp.MustCompile(`^[\?\*]+$`)

// RenderSurge renders parsed items into Surge ruleset format.
func RenderSurge(items []Item) string {
	var result []string
	var pendingIncludeComments []string
	var pendingComment string

	for _, item := range items {
		if item.Kind == ItemComment {
			trimmed := strings.TrimSpace(item.Comment)
			if strings.HasPrefix(trimmed, "# include:") {
				pendingIncludeComments = append(pendingIncludeComments, item.Comment)
			} else {
				pendingComment = item.Comment
			}
			continue
		}

		if item.Kind != ItemRule || item.Rule == nil {
			continue
		}

		line := renderSurgeRule(*item.Rule)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			pendingComment = line
			continue
		}

		if len(pendingIncludeComments) > 0 {
			result = append(result, pendingIncludeComments...)
			pendingIncludeComments = pendingIncludeComments[:0]
		}
		if pendingComment != "" {
			result = append(result, pendingComment)
			pendingComment = ""
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// RenderMihomo renders parsed items into Mihomo classical ruleset format.
// Unlike Surge, Mihomo supports DOMAIN-REGEX natively.
func RenderMihomo(items []Item) string {
	var result []string
	var pendingIncludeComments []string
	var pendingComment string

	for _, item := range items {
		if item.Kind == ItemComment {
			trimmed := strings.TrimSpace(item.Comment)
			if strings.HasPrefix(trimmed, "# include:") {
				pendingIncludeComments = append(pendingIncludeComments, item.Comment)
			} else {
				pendingComment = item.Comment
			}
			continue
		}

		if item.Kind != ItemRule || item.Rule == nil {
			continue
		}

		line := renderMihomoRule(*item.Rule)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			pendingComment = line
			continue
		}

		if len(pendingIncludeComments) > 0 {
			result = append(result, pendingIncludeComments...)
			pendingIncludeComments = pendingIncludeComments[:0]
		}
		if pendingComment != "" {
			result = append(result, pendingComment)
			pendingComment = ""
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func renderMihomoRule(rule Rule) string {
	switch rule.Kind {
	case RuleDomainSuffix:
		return appendComment("DOMAIN-SUFFIX,"+rule.Value, rule.Comment)
	case RuleDomain:
		return appendComment("DOMAIN,"+rule.Value, rule.Comment)
	case RuleDomainKeyword:
		return appendComment("DOMAIN-KEYWORD,"+rule.Value, rule.Comment)
	case RuleDomainRegex:
		// Mihomo supports DOMAIN-REGEX natively, output original regex
		return appendComment("DOMAIN-REGEX,"+rule.Value, rule.Comment)
	default:
		return appendComment(rule.Value, rule.Comment)
	}
}

// RenderEgern renders parsed items into Egern ruleset YAML.
func RenderEgern(items []Item) string {
	var domainSet []string
	var domainSuffixSet []string
	var domainKeywordSet []string
	var domainRegexSet []string

	for _, item := range items {
		if item.Kind != ItemRule || item.Rule == nil {
			continue
		}
		switch item.Rule.Kind {
		case RuleDomain:
			domainSet = append(domainSet, item.Rule.Value)
		case RuleDomainSuffix:
			domainSuffixSet = append(domainSuffixSet, item.Rule.Value)
		case RuleDomainKeyword:
			domainKeywordSet = append(domainKeywordSet, item.Rule.Value)
		case RuleDomainRegex:
			domainRegexSet = append(domainRegexSet, item.Rule.Value)
		}
	}

	var b strings.Builder
	appendSet := func(name string, values []string) {
		if len(values) == 0 {
			return
		}
		b.WriteString(name)
		b.WriteString(":\n")
		for _, value := range values {
			b.WriteString("  - ")
			b.WriteString(strconv.Quote(value))
			b.WriteString("\n")
		}
	}

	appendSet("domain_set", domainSet)
	appendSet("domain_suffix_set", domainSuffixSet)
	appendSet("domain_keyword_set", domainKeywordSet)
	appendSet("domain_regex_set", domainRegexSet)

	return strings.TrimRight(b.String(), "\n")
}

func renderSurgeRule(rule Rule) string {
	switch rule.Kind {
	case RuleDomainSuffix:
		return appendComment("DOMAIN-SUFFIX,"+rule.Value, rule.Comment)
	case RuleDomain:
		return appendComment("DOMAIN,"+rule.Value, rule.Comment)
	case RuleDomainKeyword:
		return appendComment("DOMAIN-KEYWORD,"+rule.Value, rule.Comment)
	case RuleDomainRegex:
		// Check if the regex would result in a dangerously broad wildcard
		if wildcard.IsDangerousRegex(rule.Value) {
			// Skip dangerous regex, output as comment with original value
			return appendComment("# DANGEROUS-REGEX,"+rule.Value, rule.Comment)
		}
		wildcardPattern := wildcard.RegexToWildcard(rule.Value)
		prefix := "DOMAIN-WILDCARD,"
		if skipPattern.MatchString(wildcardPattern) {
			prefix = "# SKIPPED-DOMAIN-WILDCARD,"
		}
		return appendComment(prefix+wildcardPattern, rule.Comment)
	default:
		return appendComment(rule.Value, rule.Comment)
	}
}

func appendComment(line string, comment string) string {
	if comment == "" {
		return line
	}
	if strings.HasPrefix(comment, "#") {
		return line + " " + comment
	}
	return line + " # " + comment
}
