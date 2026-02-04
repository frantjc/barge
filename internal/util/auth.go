package util

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	LocationSource      = "source"
	LocationDestination = "destination"
)

func UsernameAndPasswordForURLWithEnvFallback(u *url.URL, location, scheme string) (string, string, bool) {
	user := u.User
	u.User = nil

	if user != nil {
		if password, ok := user.Password(); ok {
			return user.Username(), password, true
		}
	}

	locSchm := strings.ToUpper(fmt.Sprintf("%s_%s", scheme, location))
	if password := os.Getenv(fmt.Sprintf("BARGE_%s_PASSWORD", locSchm)); password != "" {
		return os.Getenv(fmt.Sprintf("BARGE_%s_USERNAME", locSchm)), password, true
	}

	return "", "", false
}
