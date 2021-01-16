package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/qdm12/ddns-updater/internal/constants"
	"github.com/qdm12/ddns-updater/internal/models"
	"github.com/qdm12/ddns-updater/internal/network"
	"github.com/qdm12/ddns-updater/internal/regex"
	netlib "github.com/qdm12/golibs/network"
	"github.com/qdm12/golibs/verification"
)

type cloudflare struct {
	domain         string
	host           string
	ipVersion      models.IPVersion
	dnsLookup      bool
	key            string
	token          string
	email          string
	userServiceKey string
	zoneIdentifier string
	proxied        bool
	ttl            uint
	matcher        regex.Matcher
}

func NewCloudflare(data json.RawMessage, domain, host string, ipVersion models.IPVersion,
	noDNSLookup bool, matcher regex.Matcher) (s Settings, err error) {
	extraSettings := struct {
		Key            string `json:"key"`
		Token          string `json:"token"`
		Email          string `json:"email"`
		UserServiceKey string `json:"user_service_key"`
		ZoneIdentifier string `json:"zone_identifier"`
		Proxied        bool   `json:"proxied"`
		TTL            uint   `json:"ttl"`
	}{}
	if err := json.Unmarshal(data, &extraSettings); err != nil {
		return nil, err
	}
	c := &cloudflare{
		domain:         domain,
		host:           host,
		ipVersion:      ipVersion,
		dnsLookup:      !noDNSLookup,
		key:            extraSettings.Key,
		token:          extraSettings.Token,
		email:          extraSettings.Email,
		userServiceKey: extraSettings.UserServiceKey,
		zoneIdentifier: extraSettings.ZoneIdentifier,
		proxied:        extraSettings.Proxied,
		ttl:            extraSettings.TTL,
		matcher:        matcher,
	}
	if err := c.isValid(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *cloudflare) isValid() error {
	switch {
	case len(c.key) > 0: // email and key must be provided
		switch {
		case !c.matcher.CloudflareKey(c.key):
			return ErrMalformedKey
		case !verification.NewVerifier().MatchEmail(c.email):
			return ErrMalformedEmail
		}
	case len(c.userServiceKey) > 0: // only user service key
		if !c.matcher.CloudflareKey(c.key) {
			return ErrMalformedUserServiceKey
		}
	default: // API token only
	}
	switch {
	case len(c.zoneIdentifier) == 0:
		return ErrEmptyZoneIdentifier
	case c.ttl == 0:
		return ErrEmptyTTL
	}
	return nil
}

func (c *cloudflare) String() string {
	return toString(c.domain, c.host, constants.CLOUDFLARE, c.ipVersion)
}

func (c *cloudflare) Domain() string {
	return c.domain
}

func (c *cloudflare) Host() string {
	return c.host
}

func (c *cloudflare) IPVersion() models.IPVersion {
	return c.ipVersion
}

func (c *cloudflare) DNSLookup() bool {
	return c.dnsLookup
}

func (c *cloudflare) BuildDomainName() string {
	return buildDomainName(c.host, c.domain)
}

func (c *cloudflare) HTML() models.HTMLRow {
	return models.HTMLRow{
		Domain:    models.HTML(fmt.Sprintf("<a href=\"http://%s\">%s</a>", c.BuildDomainName(), c.BuildDomainName())),
		Host:      models.HTML(c.Host()),
		Provider:  "<a href=\"https://www.cloudflare.com\">Cloudflare</a>",
		IPVersion: models.HTML(c.ipVersion),
	}
}

func setHeaders(r *http.Request, token, userServiceKey, email, key string) {
	r.Header.Set("User-Agent", "DDNS-Updater quentin.mcgaw@gmail.com")
	switch {
	case len(token) > 0:
		r.Header.Set("Authorization", "Bearer "+token)
	case len(userServiceKey) > 0:
		r.Header.Set("X-Auth-User-Service-Key", userServiceKey)
	case len(email) > 0 && len(key) > 0:
		r.Header.Set("X-Auth-Email", email)
		r.Header.Set("X-Auth-Key", key)
	}
}

// Obtain domain identifier.
// See https://api.cloudflare.com/#dns-records-for-a-zone-list-dns-records.
func (c *cloudflare) getRecordIdentifier(ctx context.Context, client netlib.Client, newIP net.IP) (
	identifier string, upToDate bool, err error) {
	recordType := A
	if newIP.To4() == nil {
		recordType = AAAA
	}
	u := url.URL{
		Scheme: "https",
		Host:   "api.cloudflare.com",
		Path:   fmt.Sprintf("/client/v4/zones/%s/dns_records", c.zoneIdentifier),
	}
	values := url.Values{}
	values.Set("type", recordType)
	values.Set("name", c.BuildDomainName())
	values.Set("page", "1")
	values.Set("per_page", "1")
	u.RawQuery = values.Encode()
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", false, err
	}
	setHeaders(r, c.token, c.userServiceKey, c.email, c.key)
	content, status, err := client.Do(r)
	if err != nil {
		return "", false, err
	} else if status != http.StatusOK {
		return "", false, fmt.Errorf("%w: %d", ErrBadHTTPStatus, status)
	}
	listRecordsResponse := struct {
		Success bool     `json:"success"`
		Errors  []string `json:"errors"`
		Result  []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"result"`
	}{}
	if err := json.Unmarshal(content, &listRecordsResponse); err != nil {
		return "", false, fmt.Errorf("%w: %s", ErrUnmarshalResponse, err)
	}
	switch {
	case len(listRecordsResponse.Errors) > 0:
		return "", false, fmt.Errorf("%w: %s",
			ErrUnsuccessfulResponse, strings.Join(listRecordsResponse.Errors, ","))
	case !listRecordsResponse.Success:
		return "", false, ErrUnsuccessfulResponse
	case len(listRecordsResponse.Result) == 0:
		return "", false, ErrNoResultReceived
	case len(listRecordsResponse.Result) > 1:
		return "", false, fmt.Errorf("%w: %d instead of 1",
			ErrNumberOfResultsReceived, len(listRecordsResponse.Result))
	case listRecordsResponse.Result[0].Content == newIP.String(): // up to date
		return "", true, nil
	}
	return listRecordsResponse.Result[0].ID, false, nil
}

func (c *cloudflare) Update(ctx context.Context, client netlib.Client, ip net.IP) (newIP net.IP, err error) {
	recordType := A
	if ip.To4() == nil {
		recordType = AAAA
	}
	identifier, upToDate, err := c.getRecordIdentifier(ctx, client, ip)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrGetRecordIdentifier, err)
	} else if upToDate {
		return ip, nil
	}
	type cloudflarePutBody struct {
		Type    string `json:"type"`    // A or AAAA depending on ip address given
		Name    string `json:"name"`    // DNS record name i.e. example.com
		Content string `json:"content"` // ip address
		Proxied bool   `json:"proxied"` // whether the record is receiving the performance and security benefits of Cloudflare
		TTL     uint   `json:"ttl"`
	}
	u := url.URL{
		Scheme: "https",
		Host:   "api.cloudflare.com",
		Path:   fmt.Sprintf("/client/v4/zones/%s/dns_records/%s", c.zoneIdentifier, identifier),
	}
	r, err := network.BuildHTTPPut(ctx,
		u.String(),
		cloudflarePutBody{
			Type:    recordType,
			Name:    c.BuildDomainName(),
			Content: ip.String(),
			Proxied: c.proxied,
			TTL:     c.ttl,
		},
	)
	if err != nil {
		return nil, err
	}
	setHeaders(r, c.token, c.userServiceKey, c.email, c.key)
	content, status, err := client.Do(r)
	if err != nil {
		return nil, err
	} else if status > http.StatusUnsupportedMediaType {
		return nil, fmt.Errorf("%w: %d", ErrBadHTTPStatus, status)
	}
	var parsedJSON struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
		Result struct {
			Content string `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(content, &parsedJSON); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnmarshalResponse, err)
	} else if !parsedJSON.Success {
		var errStr string
		for _, e := range parsedJSON.Errors {
			errStr += fmt.Sprintf("error %d: %s; ", e.Code, e.Message)
		}
		return nil, fmt.Errorf("%w: %s", ErrUnsuccessfulResponse, errStr)
	}
	newIP = net.ParseIP(parsedJSON.Result.Content)
	if newIP == nil {
		return nil, fmt.Errorf("%w: %s", ErrIPReceivedMalformed, parsedJSON.Result.Content)
	} else if !newIP.Equal(ip) {
		return nil, fmt.Errorf("%w: %s", ErrIPReceivedMismatch, newIP.String())
	}
	return newIP, nil
}
