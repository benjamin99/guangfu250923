package middleware

import (
	"errors"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// IPFilter (formerly CountryFilter) restricts POST/PATCH requests based on Cloudflare-provided country header.
// Despite the name change, current implementation still uses Cf-Ipcountry. If you later need pure IP allowlisting,
// extend here to parse client IP (see middleware.clientIP) and compare with an ALLOWED_IPS list.
//
// Environment variables:
//
//	ALLOWED_COUNTRIES   comma-separated list, case-insensitive (e.g. "TW,JP")
//	ALLOW_NO_COUNTRY    if "true", missing Cf-Ipcountry header is allowed
//	COUNTRY_BLOCK_MSG   custom rejection message (default: country not allowed)
//	ALLOWED_IPS         optional comma-separated list of IPs or CIDRs (e.g. "1.2.3.4,10.0.0.0/8,2001:db8::/32"). If present, client IP must fall inside one of them.
//
// Behavior:
//   - Only affects POST & PATCH.
//   - If ALLOWED_COUNTRIES unset/empty => no-op.
//   - 403 on disallowed or missing (unless ALLOW_NO_COUNTRY=true).
func IPFilter() gin.HandlerFunc {
	// Country list (optional)
	allowedCountriesRaw := os.Getenv("ALLOWED_COUNTRIES")
	allowSet := map[string]struct{}{}
	if strings.TrimSpace(allowedCountriesRaw) != "" {
		for _, p := range strings.Split(allowedCountriesRaw, ",") {
			p = strings.TrimSpace(strings.ToUpper(p))
			if p == "" {
				continue
			}
			allowSet[p] = struct{}{}
		}
	}

	allowNoHeader := strings.EqualFold(os.Getenv("ALLOW_NO_COUNTRY"), "true")
	blockMsg := os.Getenv("COUNTRY_BLOCK_MSG")
	if blockMsg == "" {
		blockMsg = "country not allowed"
	}

	// IP/CIDR list (optional)
	allowedIPsRaw := os.Getenv("ALLOWED_IPS")
	var ipNets []*net.IPNet
	if strings.TrimSpace(allowedIPsRaw) != "" {
		for _, token := range strings.Split(allowedIPsRaw, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			if strings.Contains(token, "/") {
				if _, network, err := net.ParseCIDR(token); err == nil {
					ipNets = append(ipNets, network)
				}
				continue
			}
			// Single IP -> convert to /32 or /128
			ip := net.ParseIP(token)
			if ip == nil {
				continue
			}
			var mask net.IPMask
			if ip.To4() != nil {
				mask = net.CIDRMask(32, 32)
			} else {
				mask = net.CIDRMask(128, 128)
			}
			ipNet := &net.IPNet{IP: ip, Mask: mask}
			ipNets = append(ipNets, ipNet)
		}
	}

	// Fast no-op if neither countries nor IPs configured
	if len(allowSet) == 0 && len(ipNets) == 0 {
		return func(c *gin.Context) { c.Next() }
	}

	isIPAllowed := func(ipStr string) bool {
		if len(ipNets) == 0 {
			return true // no IP constraint
		}
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return false
		}
		for _, n := range ipNets {
			if n.Contains(ip) {
				return true
			}
		}
		return false
	}

	block := func(c *gin.Context, reason string, details gin.H) {
		// Attach a gin error so RequestLogger captures it uniformly.
		c.Error(errors.New("blocked: " + reason)) //nolint:errcheck
		payload := gin.H{"error": blockMsg, "reason": reason}
		for k, v := range details {
			payload[k] = v
		}
		c.JSON(http.StatusForbidden, payload)
		c.Abort()
	}

	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPatch {
			c.Next()
			return
		}

		// Country enforcement (if configured)
		if len(allowSet) > 0 {
			country := c.GetHeader("Cf-Ipcountry")
			if country == "" {
				if !allowNoHeader {
					block(c, "missing Cf-Ipcountry", gin.H{})
					return
				}
			} else {
				country = strings.ToUpper(strings.TrimSpace(country))
				if _, ok := allowSet[country]; !ok {
					block(c, "disallowed country", gin.H{"country": country})
					return
				}
			}
		}

		// IP enforcement (if configured)
		if len(ipNets) > 0 {
			cip := clientIP(c)
			if !isIPAllowed(cip) {
				block(c, "ip not allowed", gin.H{"ip": cip})
				return
			}
		}

		c.Next()
	}
}
