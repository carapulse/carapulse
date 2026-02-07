package tools

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type egressProxy struct {
	allowlist []string
	ln        net.Listener
	wg        sync.WaitGroup
}

func startEgressProxy(allowlist []string) (*egressProxy, error) {
	if len(allowlist) == 0 {
		return nil, nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := &egressProxy{allowlist: allowlist, ln: ln}
	p.wg.Add(1)
	go p.serve()
	return p, nil
}

func (p *egressProxy) addr() string {
	if p == nil || p.ln == nil {
		return ""
	}
	return p.ln.Addr().String()
}

func (p *egressProxy) close() {
	if p == nil {
		return
	}
	if p.ln != nil {
		_ = p.ln.Close()
	}
	p.wg.Wait()
}

func (p *egressProxy) serve() {
	defer p.wg.Done()
	for {
		conn, err := p.ln.Accept()
		if err != nil {
			return
		}
		go p.handleConn(conn)
	}
}

func (p *egressProxy) handleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	host := req.Host
	if req.Method == http.MethodConnect {
		if !p.allowHost(host) {
			_, _ = io.WriteString(conn, "HTTP/1.1 403 Forbidden\r\n\r\n")
			return
		}
		target, err := net.Dial("tcp", host)
		if err != nil {
			_, _ = io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
			return
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		go func() { _, _ = io.Copy(target, reader); _ = target.Close() }()
		_, _ = io.Copy(conn, target)
		_ = target.Close()
		return
	}
	if req.URL == nil {
		return
	}
	if req.URL.Host == "" && host != "" {
		req.URL.Host = host
	}
	if !p.allowHost(req.URL.Host) {
		_, _ = io.WriteString(conn, "HTTP/1.1 403 Forbidden\r\n\r\n")
		return
	}
	req.RequestURI = ""
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		_, _ = io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer resp.Body.Close()
	_ = resp.Write(conn)
}

func (p *egressProxy) allowHost(raw string) bool {
	host := strings.TrimSpace(raw)
	if host == "" {
		return false
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return false
	}
	if u.Hostname() == "" {
		return false
	}
	return allowHost(fmt.Sprintf("http://%s", u.Host), p.allowlist)
}

func proxyEnv(allowlist []string, runtime string) (map[string]string, func(), error) {
	if len(allowlist) == 0 {
		return nil, func() {}, nil
	}
	proxy, err := startEgressProxy(allowlist)
	if err != nil {
		return nil, func() {}, err
	}
	addr := proxy.addr()
	if addr == "" {
		proxy.close()
		return nil, func() {}, errors.New("proxy addr required")
	}
	host := "127.0.0.1"
	if strings.EqualFold(strings.TrimSpace(runtime), "docker") {
		host = "host.docker.internal"
	}
	if strings.EqualFold(strings.TrimSpace(runtime), "podman") {
		host = "host.containers.internal"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		proxy.close()
		return nil, func() {}, err
	}
	proxyURL := fmt.Sprintf("http://%s:%s", host, port)
	env := map[string]string{
		"HTTP_PROXY":  proxyURL,
		"HTTPS_PROXY": proxyURL,
		"ALL_PROXY":   proxyURL,
	}
	return env, proxy.close, nil
}
