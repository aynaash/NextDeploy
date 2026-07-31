package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aynaash/nextdeploy/shared/config"
)

// environmentPattern constrains --environment to what Cloudflare resource
// names tolerate. The value is concatenated into worker/bucket names, so an
// unconstrained string would either be silently mangled by sanitizeCFName —
// making `ship -e Foo/Bar` and `ship -e foo-bar` collide on the same stack —
// or produce a name the API rejects deep into the deploy.
var environmentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}[a-z0-9]$|^[a-z0-9]$`)

// applyEnvironmentOverride points cfg at a named environment (e.g. "pr-42",
// "staging"). Empty leaves the config's own value untouched, which the
// Cloudflare provider defaults to "production".
//
// One override point, not N. It's tempting to thread an environment parameter
// through workerName / bucketNameFromApp / Initialize, but the provider
// already reads cfg.App.Environment as its single source — overriding it once
// here means every downstream resource name follows automatically, with no
// chance of one resource landing in the wrong namespace.
func applyEnvironmentOverride(cfg *config.NextDeployConfig, environment string) error {
	if environment == "" {
		return nil
	}
	env := strings.TrimSpace(environment)
	if !environmentPattern.MatchString(env) {
		return fmt.Errorf(
			"invalid --environment %q: use lowercase letters, digits and hyphens (1-32 chars, "+
				"not starting or ending with a hyphen), e.g. pr-42 or staging", environment)
	}
	cfg.App.Environment = env
	return nil
}
