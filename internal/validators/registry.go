package validators


import "strings"

func IsValidRegistry(registry strings) bool {
	return !strings.Contains(registry, " ") &&
	!strings.HasPrefix(registry, "/") &&
	!strings.HasSuffix(registry, "/")
}
