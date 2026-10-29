package npmprofile

import (
	"net/url"
	"strings"
)

func escapeLockfileFragment(lockfilePath string) string {
	segments := strings.Split(lockfilePath, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}
