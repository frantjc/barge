package chartmuseum

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"

	"github.com/frantjc/barge"
	"github.com/frantjc/barge/internal/utils"
	"helm.sh/helm/v3/pkg/chart"
)

func init() {
	barge.RegisterDestination(
		new(destination),
		"chartmuseum",
		"cm",
	)
}

type destination struct{}

func (d *destination) Write(ctx context.Context, u *url.URL, c *chart.Chart) error {
	q := u.Query()
	if insecure, _ := strconv.ParseBool(q.Get("insecure")); insecure {
		u.Scheme = "http"
	} else {
		u.Scheme = "https"
	}

	// TODO(frantjc): Use io.Pipe here.
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)
	fw, err := mw.CreateFormFile("chart", fmt.Sprintf("%s-%s.tgz", c.Name(), c.Metadata.Version))
	if err != nil {
		return err
	}

	rc, err := utils.WriteChartToArchive(c)
	if err != nil {
		return err
	}
	defer rc.Close()

	if _, err := io.Copy(fw, rc); err != nil {
		return err
	}

	if err := mw.Close(); err != nil {
		return err
	}

	user := u.User
	u.User = nil

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.JoinPath("/api/charts").String(), buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", mw.FormDataContentType())

	if user != nil {
		if password, ok := user.Password(); ok {
			username := user.Username()
			req.Header.Add(
				"Authorization",
				fmt.Sprintf(
					"Basic %s",
					base64.RawURLEncoding.EncodeToString(
						[]byte(fmt.Sprintf("%s:%s", username, password)),
					),
				),
			)
		}
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if statusCode := res.StatusCode; 200 > statusCode || statusCode >= 400 {
		return fmt.Errorf("http status %s", res.Status)
	}

	return nil
}
