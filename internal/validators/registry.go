package validators


import "strings"

func IsValidRegistry(registry string) bool {
	return !strings.Contains(registry, " ") &&
	!strings.HasPrefix(registry, "/") &&
	!strings.HasSuffix(registry, "/")
}
