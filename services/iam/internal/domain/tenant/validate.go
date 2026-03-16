package tenant

import "regexp"

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func isValidSlug(slug string) bool {
	return slugRegex.MatchString(slug)
}
