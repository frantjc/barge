package utils

import (
	"net/url"
	"path"
	"strconv"

	"helm.sh/helm/v3/pkg/registry"
)

func NewRegistryClientFromURL(u *url.URL) (*registry.Client, error) {
	opts := []registry.ClientOption{}
	registry.ClientOptPlainHTTP()
	if user := u.User; user != nil {
		if password, ok := user.Password(); ok {
			username := user.Username()
			opts = append(opts, registry.ClientOptBasicAuth(username, password))
		}
	}
	if insecure, _ := strconv.ParseBool(u.Query().Get("insecure")); insecure {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}
	return registry.NewClient(opts...)
}

func RefFromURL(u *url.URL) string {
	return path.Join(u.Host, u.Path)
}
