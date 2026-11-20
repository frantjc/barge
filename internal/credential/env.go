package credential

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Env finds credentials in BARGE_<SCHEME>_<LOCATION>_USERNAME and
// BARGE_<SCHEME>_<LOCATION>_PASSWORD, or a bearer token in
// BARGE_<SCHEME>_<LOCATION>_TOKEN, e.g. BARGE_ARTIFACTORY_DESTINATION_TOKEN,
// trying each of the Request's Schemes in order. For a given scheme, a
// password takes precedence over a token.
type Env struct{}

func (Env) String() string {
	return "environment"
}

var envReplacer = strings.NewReplacer("+", "_", "-", "_", ".", "_")

func (Env) Credential(_ context.Context, req *Request) (*Credential, error) {
	for _, scheme := range req.Schemes {
		prefix := strings.ToUpper(envReplacer.Replace(fmt.Sprintf("BARGE_%s_%s", scheme, req.Location)))
		if password := os.Getenv(prefix + "_PASSWORD"); password != "" {
			return &Credential{Username: os.Getenv(prefix + "_USERNAME"), Password: password}, nil
		}

		if token := os.Getenv(prefix + "_TOKEN"); token != "" {
			return &Credential{Token: token}, nil
		}
	}

	return nil, nil
}
