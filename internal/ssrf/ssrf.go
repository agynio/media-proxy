package ssrf

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
)

var defaultDenyCIDRs = []string{
	"127.0.0.0/8",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"169.254.0.0/16",
	"::1/128",
	"fe80::/10",
	"fc00::/7",
	"::ffff:0:0/96",
}

type Checker struct {
	networks []*net.IPNet
}

type DeniedError struct {
	IP net.IP
}

func (e DeniedError) Error() string {
	return fmt.Sprintf("connection denied to %s", e.IP.String())
}

func DefaultChecker() (*Checker, error) {
	networks := make([]*net.IPNet, 0, len(defaultDenyCIDRs))
	for _, cidr := range defaultDenyCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid deny cidr %q: %w", cidr, err)
		}
		networks = append(networks, network)
	}
	return &Checker{networks: networks}, nil
}

func (c *Checker) NewDialer() *net.Dialer {
	return &net.Dialer{Control: c.Control}
}

func (c *Checker) Control(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" {
		return fmt.Errorf("missing host in address")
	}
	if idx := strings.Index(host, "%"); idx != -1 {
		host = host[:idx]
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", host)
	}
	if !strings.Contains(host, ":") {
		if ipv4 := ip.To4(); ipv4 != nil {
			ip = ipv4
		}
	}
	if c.Denied(ip) {
		return DeniedError{IP: ip}
	}
	return nil
}

func (c *Checker) Denied(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ipLen := len(ip)
	for _, network := range c.networks {
		if ipLen == net.IPv4len && len(network.IP) == net.IPv6len {
			continue
		}
		if ipLen == net.IPv6len && len(network.IP) == net.IPv4len {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func IsDeniedError(err error) bool {
	var denied DeniedError
	return errors.As(err, &denied)
}
