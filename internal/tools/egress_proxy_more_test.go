package tools

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEgressProxyDeniedRequest(t *testing.T) {
	proxy, err := startEgressProxy([]string{"allowed.com"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer proxy.close()
	conn, err := net.Dial("tcp", proxy.addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET http://blocked.com/ HTTP/1.1\r\nHost: blocked.com\r\n\r\n")
	resp, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(resp), "403") {
		t.Fatalf("resp: %s", string(resp))
	}
}

func TestEgressProxyAllowedRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	proxy, err := startEgressProxy([]string{"127.0.0.1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer proxy.close()
	conn, err := net.Dial("tcp", proxy.addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET http://%s/ HTTP/1.1\r\nHost: %s\r\n\r\n", host, host)
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(line, "200") {
		t.Fatalf("line: %s", line)
	}
}

func TestEgressProxyConnectAllowed(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer target.Close()
	received := make(chan struct{}, 1)
	go func() {
		conn, err := target.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4)
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, _ = conn.Read(buf)
		received <- struct{}{}
	}()
	proxy, err := startEgressProxy([]string{"127.0.0.1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer proxy.close()
	conn, err := net.Dial("tcp", proxy.addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	targetAddr := target.Addr().String()
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", targetAddr, targetAddr)
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(line, "200") {
		t.Fatalf("line: %s", line)
	}
	for {
		l, err := reader.ReadString('\n')
		if err != nil || l == "\r\n" {
			break
		}
	}
	_, _ = conn.Write([]byte("ping"))
	select {
	case <-received:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("no target data")
	}
}
