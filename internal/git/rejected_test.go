package git

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	github "github.com/google/go-github/v72/github"
	"github.com/stretchr/testify/require"
)

func gitHubError(status int) error {
	return &github.ErrorResponse{Response: &http.Response{StatusCode: status}}
}

func TestIsRejected(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		err             error
		withCredentials bool
		want            bool
	}{
		{"nil", nil, true, false},
		{"git authentication required", fmt.Errorf("clone: %w", transport.ErrAuthenticationRequired), false, true},
		{"git authorization failed", transport.ErrAuthorizationFailed, true, true},
		{"tarball 401", fmt.Errorf("get github tarball: 401 Unauthorized: %w", errUnauthorized), true, true},
		{"github 401", gitHubError(http.StatusUnauthorized), true, true},
		// GitHub hides private repositories from tokens that can't see them.
		{"github 404 with credentials", gitHubError(http.StatusNotFound), true, true},
		{"github 404 without credentials", gitHubError(http.StatusNotFound), false, false},
		{"github 500", gitHubError(http.StatusInternalServerError), true, false},
		{"other", errors.New("boom"), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isRejected(tc.err, tc.withCredentials))
		})
	}
}
