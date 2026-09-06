package metadataservice

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Metadata struct {
	Title       string
	Description string
	Image       string
	Favicon     string
	SiteName    string
}

const (
	fetchTimeout = 6 * time.Second
	maxRedirects = 5
	maxBodyBytes = 2 << 20 // 2MB
)

// isBlockedIP rejects loopback, private, link-local, and other non-public ranges
// so the server can't be tricked into fetching internal network resources (SSRF).
func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// safeDialContext resolves the hostname itself and dials the resolved IP directly,
// so the safety check can't be bypassed by DNS rebinding between check and connect.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}

	for _, ip := range ips {
		if isBlockedIP(ip) {
			return nil, customErrInvalidURL
		}
	}

	dialer := &net.Dialer{Timeout: fetchTimeout}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

var customErrInvalidURL = errors.New("disallowed host")

func newSafeClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: safeDialContext,
		},
		Timeout: fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
}

func validateURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return nil, errors.New("invalid URL")
	}

	return parsed, nil
}

func FetchMetadata(ctx context.Context, rawURL string) (*Metadata, error) {
	parsed, err := validateURL(rawURL)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SaveTheLinkBot/1.0)")
	req.Header.Set("Accept", "text/html")

	resp, err := newSafeClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, errors.New("failed to fetch URL")
	}

	doc, err := html.Parse(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}

	return extractMetadata(doc, resp.Request.URL), nil
}

func extractMetadata(doc *html.Node, base *url.URL) *Metadata {
	var title, ogTitle, ogDescription, ogImage, ogSiteName, metaDescription, iconHref string

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "title":
				if title == "" && node.FirstChild != nil {
					title = node.FirstChild.Data
				}
			case "meta":
				content := attr(node, "content")
				switch attr(node, "property") {
				case "og:title":
					ogTitle = content
				case "og:description":
					ogDescription = content
				case "og:image":
					ogImage = content
				case "og:site_name":
					ogSiteName = content
				}
				if attr(node, "name") == "description" && metaDescription == "" {
					metaDescription = content
				}
			case "link":
				rel := attr(node, "rel")
				if iconHref == "" && (rel == "icon" || rel == "shortcut icon") {
					iconHref = attr(node, "href")
				}
			}
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	favicon := resolveURL(base, iconHref)
	if favicon == "" {
		favicon = resolveURL(base, "/favicon.ico")
	}

	return &Metadata{
		Title:       firstNonEmpty(ogTitle, title),
		Description: firstNonEmpty(ogDescription, metaDescription),
		SiteName:    firstNonEmpty(ogSiteName, base.Hostname()),
		Image:       resolveURL(base, ogImage),
		Favicon:     favicon,
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func resolveURL(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}

	parsed, err := url.Parse(ref)
	if err != nil {
		return ""
	}

	return base.ResolveReference(parsed).String()
}
