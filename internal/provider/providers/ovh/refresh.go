package ovh

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (p *Provider) refresh(ctx context.Context, client *http.Client, timestamp int64) (err error) {
	u := url.URL{
		Scheme: p.apiURL.Scheme,
		Host:   p.apiURL.Host,
		Path:   fmt.Sprintf("%s/domain/zone/%s/refresh", p.apiURL.Path, p.domain),
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return fmt.Errorf("creating http request: %w", err)
	}
	p.setHeaderCommon(request.Header)
	p.setHeaderAuth(request.Header, timestamp, request.Method, request.URL, nil)

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("doing http request: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return extractAPIError(response)
	}

	_ = response.Body.Close()

	return nil
}
