package util

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func RefFromURL(u *url.URL) string {
	ref := path.Join(u.Host, u.Path)
	if strings.Contains(u.Path, ":") {
		return ref
	}
	tag := u.Query().Get("version")
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s:%s", ref, tag)
}
