package daemon

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/aynaash/nextdeploy/shared/nextcore"
)

func (s *SidecarHandler) middlewareMayMatch(r *http.Request, requestPath string) bool {
	if !s.hasMiddleware {
		return false
	}
	if s.meta.Middleware == nil || len(s.meta.Middleware.Matchers) == 0 {
		return true
	}
	for _, matcher := range s.meta.Middleware.Matchers {
		matches, known := middlewarePathMatches(matcher.Pathname, matcher.Pattern, requestPath)
		if !known {
			return true
		}
		if !matches {
			continue
		}
		conditionsMatch := true
		for _, condition := range matcher.Has {
			ok, supported := middlewareConditionMatches(condition, r)
			if !supported {
				return true
			}
			conditionsMatch = conditionsMatch && ok
		}
		for _, condition := range matcher.Missing {
			ok, supported := middlewareConditionMatches(condition, r)
			if !supported {
				return true
			}
			conditionsMatch = conditionsMatch && !ok
		}
		if conditionsMatch {
			return true
		}
	}
	return false
}

func middlewarePathMatches(pathname, pattern, requestPath string) (bool, bool) {
	if pattern != "" {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return true, false
		}
		return re.MatchString(requestPath), true
	}
	if pathname == "" {
		return true, false
	}
	if matches, known := nextNegativeLookaheadMatches(pathname, requestPath); known {
		return matches, true
	}
	expr := regexp.QuoteMeta(pathname)
	param := regexp.MustCompile(`:[^/\\]+(?:\\[?*+])?`)
	expr = param.ReplaceAllStringFunc(expr, func(token string) string {
		token = strings.ReplaceAll(token, `\`, "")
		switch {
		case strings.HasSuffix(token, "*"):
			return `[^/]+(?:/.*)?`
		case strings.HasSuffix(token, "+"):
			return `[^/]+(?:/[^/]+)*`
		case strings.HasSuffix(token, "?"):
			return `[^/]*`
		default:
			return `[^/]+`
		}
	})
	expr = strings.ReplaceAll(expr, `\*\*`, `.*`)
	expr = strings.ReplaceAll(expr, `\*`, `[^/]*`)
	re, err := regexp.Compile("^" + expr + "$")
	if err != nil {
		return true, false
	}
	return re.MatchString(requestPath), true
}

func nextNegativeLookaheadMatches(pathname, requestPath string) (bool, bool) {
	const prefix = "/((?!"
	const suffix = ").*)"
	if !strings.HasPrefix(pathname, prefix) || !strings.HasSuffix(pathname, suffix) {
		return false, false
	}
	expression := strings.TrimSuffix(strings.TrimPrefix(pathname, prefix), suffix)
	alternatives, ok := splitRegexAlternatives(expression)
	if !ok || len(alternatives) == 0 {
		return true, false
	}
	path := strings.TrimPrefix(requestPath, "/")
	for _, alternative := range alternatives {
		exclusion, err := regexp.Compile("^(?:" + alternative + ")")
		if err != nil {
			return true, false
		}
		if exclusion.MatchString(path) {
			return false, true
		}
	}
	return true, true
}

func splitRegexAlternatives(expression string) ([]string, bool) {
	var (
		alternatives []string
		start        int
		groupDepth   int
		classDepth   int
		escaped      bool
	)
	for i, char := range expression {
		if escaped {
			escaped = false
			continue
		}
		switch {
		case char == '\\':
			escaped = true
		case char == '[':
			classDepth++
		case char == ']' && classDepth > 0:
			classDepth--
		case classDepth == 0 && char == '(':
			groupDepth++
		case classDepth == 0 && char == ')' && groupDepth > 0:
			groupDepth--
		case classDepth == 0 && groupDepth == 0 && char == '|':
			alternatives = append(alternatives, expression[start:i])
			start = i + 1
		}
	}
	if escaped || classDepth != 0 || groupDepth != 0 {
		return nil, false
	}
	alternatives = append(alternatives, expression[start:])
	for _, alternative := range alternatives {
		if alternative == "" {
			return nil, false
		}
	}
	return alternatives, true
}

func middlewareConditionMatches(condition nextcore.MiddlewareCondition, r *http.Request) (bool, bool) {
	var value string
	switch strings.ToLower(condition.Type) {
	case "header":
		value = r.Header.Get(condition.Key)
	case "cookie":
		cookie, err := r.Cookie(condition.Key)
		if err != nil {
			return false, true
		}
		value = cookie.Value
	case "query":
		value = r.URL.Query().Get(condition.Key)
	case "host":
		value = r.Host
	default:
		return false, false
	}
	if condition.Value == "" {
		return value != "", true
	}
	re, err := regexp.Compile(condition.Value)
	if err == nil {
		return re.MatchString(value), true
	}
	return false, false
}
