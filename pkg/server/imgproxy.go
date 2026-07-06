package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxImage = 5 << 20

// imgClient refuses to connect to loopback, private, and link-local
// addresses after DNS resolution, so a hostile card thumbnail cannot point
// the proxy at anything internal.
var imgClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if ip.IP.IsLoopback() || ip.IP.IsPrivate() || ip.IP.IsLinkLocalUnicast() || ip.IP.IsUnspecified() {
					return nil, fmt.Errorf("refusing internal address")
				}
			}
			var d net.Dialer
			return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

// handleImg proxies card thumbnails so the browser never hands third-party
// image hosts the user's IP and referer.
func (s *Server) handleImg(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("u")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeError(w, http.StatusBadRequest, "bad image url")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad image url")
		return
	}
	req.Header.Set("User-Agent", "shirabe/1.0")
	resp, err := imgClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch failed")
		return
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(ct, "image/") {
		writeError(w, http.StatusBadGateway, "not an image")
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, io.LimitReader(resp.Body, maxImage))
}
