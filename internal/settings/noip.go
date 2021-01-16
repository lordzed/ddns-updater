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
	netlib "github.com/qdm12/golibs/network"
	"github.com/qdm12/golibs/verification"
)

type noip struct {
	domain        string
	host          string
	ipVersion     models.IPVersion
	dnsLookup     bool
	username      string
	password      string
	useProviderIP bool
}

func NewNoip(data json.RawMessage, domain, host string, ipVersion models.IPVersion,
	noDNSLookup bool, matcher regex.Matcher) (s Settings, err error) {
	extraSettings := struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		UseProviderIP bool   `json:"provider_ip"`
	}{}
	if err := json.Unmarshal(data, &extraSettings); err != nil {
		return nil, err
	}
	n := &noip{
		domain:        domain,
		host:          host,
		ipVersion:     ipVersion,
		dnsLookup:     !noDNSLookup,
		username:      extraSettings.Username,
		password:      extraSettings.Password,
		useProviderIP: extraSettings.UseProviderIP,
	}
	if err := n.isValid(); err != nil {
		return nil, err
	}
	return n, nil
}

func (n *noip) isValid() error {
	const maxUsernameLength = 50
	switch {
	case len(n.username) == 0:
		return ErrEmptyUsername
	case len(n.username) > maxUsernameLength:
		return fmt.Errorf("%w: longer than 50 characters", ErrMalformedUsername)
	case len(n.password) == 0:
		return ErrEmptyPassword
	case n.host == "*":
		return ErrHostWildcard
	}
	return nil
}

func (n *noip) String() string {
	return toString(n.domain, n.host, constants.NOIP, n.ipVersion)
}

func (n *noip) Domain() string {
	return n.domain
}

func (n *noip) Host() string {
	return n.host
}

func (n *noip) DNSLookup() bool {
	return n.dnsLookup
}

func (n *noip) IPVersion() models.IPVersion {
	return n.ipVersion
}

func (n *noip) BuildDomainName() string {
	return buildDomainName(n.host, n.domain)
}

func (n *noip) HTML() models.HTMLRow {
	return models.HTMLRow{
		Domain:    models.HTML(fmt.Sprintf("<a href=\"http://%s\">%s</a>", n.BuildDomainName(), n.BuildDomainName())),
		Host:      models.HTML(n.Host()),
		Provider:  "<a href=\"https://www.noip.com/\">NoIP</a>",
		IPVersion: models.HTML(n.ipVersion),
	}
}

func (n *noip) Update(ctx context.Context, client netlib.Client, ip net.IP) (newIP net.IP, err error) {
	u := url.URL{
		Scheme: "https",
		Host:   "dynupdate.no-ip.com",
		Path:   "/nic/update",
		User:   url.UserPassword(n.username, n.password),
	}
	values := url.Values{}
	values.Set("hostname", n.BuildDomainName())
	if !n.useProviderIP {
		if ip.To4() == nil {
			values.Set("myipv6", ip.String())
		} else {
			values.Set("myip", ip.String())
		}
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
	switch s {
	case "":
		if status != http.StatusOK {
			return nil, fmt.Errorf("%w: %d", ErrBadHTTPStatus, status)
		}
	case nineoneone:
		return nil, ErrDNSServerSide
	case abuse:
		return nil, ErrAbuse
	case "!donator":
		return nil, ErrFeatureUnavailable
	case badagent:
		return nil, ErrBannedUserAgent
	case badauth:
		return nil, ErrAuth
	case nohost:
		return nil, ErrHostnameNotExists
	}
	if strings.Contains(s, "nochg") || strings.Contains(s, "good") {
		ips := verification.NewVerifier().SearchIPv4(s)
		if ips == nil {
			return nil, ErrNoResultReceived
		}
		newIP = net.ParseIP(ips[0])
		if newIP == nil {
			return nil, fmt.Errorf("%w: %s", ErrIPReceivedMalformed, ips[0])
		}
		if !n.useProviderIP && !ip.Equal(newIP) {
			return nil, fmt.Errorf("%w: %s", ErrIPReceivedMismatch, newIP.String())
		}
		return newIP, nil
	}
	return nil, ErrUnknownResponse
}
