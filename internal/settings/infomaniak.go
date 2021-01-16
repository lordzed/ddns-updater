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
	"github.com/qdm12/ddns-updater/internal/regex"
	"github.com/qdm12/golibs/network"
)

type infomaniak struct {
	domain        string
	host          string
	ipVersion     models.IPVersion
	dnsLookup     bool
	username      string
	password      string
	useProviderIP bool
}

func NewInfomaniak(data json.RawMessage, domain, host string, ipVersion models.IPVersion,
	noDNSLookup bool, matcher regex.Matcher) (s Settings, err error) {
	extraSettings := struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		UseProviderIP bool   `json:"provider_ip"`
	}{}
	if err := json.Unmarshal(data, &extraSettings); err != nil {
		return nil, err
	}
	i := &infomaniak{
		domain:        domain,
		host:          host,
		ipVersion:     ipVersion,
		dnsLookup:     !noDNSLookup,
		username:      extraSettings.Username,
		password:      extraSettings.Password,
		useProviderIP: extraSettings.UseProviderIP,
	}
	if err := i.isValid(); err != nil {
		return nil, err
	}
	return i, nil
}

func (i *infomaniak) isValid() error {
	switch {
	case len(i.username) == 0:
		return ErrEmptyUsername
	case len(i.password) == 0:
		return ErrEmptyPassword
	case i.host == "*":
		return ErrHostWildcard
	}
	return nil
}

func (i *infomaniak) String() string {
	return toString(i.domain, i.host, constants.INFOMANIAK, i.ipVersion)
}

func (i *infomaniak) Domain() string {
	return i.domain
}

func (i *infomaniak) Host() string {
	return i.host
}

func (i *infomaniak) IPVersion() models.IPVersion {
	return i.ipVersion
}

func (i *infomaniak) DNSLookup() bool {
	return i.dnsLookup
}

func (i *infomaniak) BuildDomainName() string {
	return buildDomainName(i.host, i.domain)
}

func (i *infomaniak) HTML() models.HTMLRow {
	return models.HTMLRow{
		Domain:    models.HTML(fmt.Sprintf("<a href=\"http://%s\">%s</a>", i.BuildDomainName(), i.BuildDomainName())),
		Host:      models.HTML(i.Host()),
		Provider:  "<a href=\"https://www.infomaniak.com/\">Infomaniak</a>",
		IPVersion: models.HTML(i.ipVersion),
	}
}

func (i *infomaniak) Update(ctx context.Context, client network.Client, ip net.IP) (newIP net.IP, err error) {
	u := url.URL{
		Scheme: "https",
		Host:   "infomaniak.com",
		Path:   "/nic/update",
		User:   url.UserPassword(i.username, i.password),
	}
	values := url.Values{}
	values.Set("hostname", i.domain)
	if i.host != "@" {
		values.Set("hostname", i.host+"."+i.domain)
	}
	if !i.useProviderIP {
		values.Set("myip", ip.String())
	}
	u.RawQuery = values.Encode()
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("User-Agent", "DDNS-Updater quentin.mcgaw@gmail.com")
	content, status, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	s := string(content)
	switch status {
	case http.StatusOK:
		switch {
		case strings.HasPrefix(s, "good "):
			newIP = net.ParseIP(s[5:])
			if newIP == nil {
				return nil, fmt.Errorf("%w: %s", ErrIPReceivedMalformed, s)
			} else if ip != nil && !ip.Equal(newIP) {
				return nil, fmt.Errorf("%w: %s", ErrIPReceivedMismatch, newIP)
			}
			return newIP, nil
		case strings.HasPrefix(s, "nochg "):
			newIP = net.ParseIP(s[6:])
			if newIP == nil {
				return nil, fmt.Errorf("%w: in response %q", ErrNoResultReceived, s)
			} else if ip != nil && !ip.Equal(newIP) {
				return nil, fmt.Errorf("%w: %s", ErrIPReceivedMismatch, newIP)
			}
			return newIP, nil
		default:
			return nil, fmt.Errorf("%w: %s", ErrUnknownResponse, s)
		}
	case http.StatusBadRequest:
		switch s {
		case nohost:
			return nil, ErrHostnameNotExists
		case badauth:
			return nil, ErrAuth
		default:
			return nil, fmt.Errorf("%w: %d", ErrBadHTTPStatus, status)
		}
	default:
		return nil, fmt.Errorf("%w: %d: %s", ErrBadHTTPStatus, status, s)
	}
}
