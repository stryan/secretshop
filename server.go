package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log"
	"mime"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "gemini context value " + k.name
}

var (
	// ServerContextKey is a context key. It can be used in Gemini
	// handlers with context.WithValue to access the server that
	// started the handler. The associated value will be of type *Server.
	ServerContextKey = &contextKey{"gemini-server"}

	// LocalAddrContextKey is a context key. It can be used in
	// Gemini handlers with context.WithValue to access the address
	// the local address the connection arrived on.
	// The associated value will be of type net.Addr.
	LocalAddrContextKey = &contextKey{"local-addr"}
)

type Server struct {
	Addr string // TCP address to listen on, ":gemini" if empty
	Port string // TCP port

	HostnameToRoot map[string]string //FQDN hostname to root folder
	HostnameToCGI  map[string]string //FQDN hostname to CGI folder
}

type conn struct {
	server   *Server
	C        net.Conn
	tlsState *tls.ConnectionState
}

func (s *Server) newConn(rwc net.Conn) *conn {
	c := &conn{
		server:   s,
		C:        rwc,
		tlsState: nil,
	}
	return c
}

func ListenAndServeTLS(port string, cps []GeminiConfig) error {
	server := &Server{Addr: ":" + port, Port: port, HostnameToRoot: make(map[string]string), HostnameToCGI: make(map[string]string)}
	for _, c := range cps {
		server.HostnameToRoot[c.Hostname] = c.RootDir
		server.HostnameToCGI[c.Hostname] = c.CGIDir
	}
	return server.ListenAndServeTLS(cps)
}

func (s *Server) ListenAndServeTLS(configs []GeminiConfig) error {
	addr := s.Addr
	mime.AddExtensionType(".gmi", "text/gemini")
	mime.AddExtensionType(".gemini", "text/gemini")
	if addr == "" {
		addr = ":1965"
	}
	certs := make([]tls.Certificate, len(configs))
	for i, c := range configs {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			log.Fatalf("Error loading certs: %s", err)
		}
		certs[i] = cert
	}

	config := tls.Config{Certificates: certs}
	config.Rand = rand.Reader

	ln, err := tls.Listen("tcp", addr, &config)
	if err != nil {
		log.Fatalf("server: listen: %s", err)
	}

	return s.Serve(ln)
}

func (s *Server) Serve(l net.Listener) error {
	defer l.Close()

	ctx := context.Background()
	ctx = context.WithValue(ctx, ServerContextKey, s)
	ctx = context.WithValue(ctx, LocalAddrContextKey, l.Addr())

	for {
		rw, err := l.Accept()
		if err != nil {
			fmt.Errorf("error accepting new client: %s", err)
			return err
		}

		c := s.newConn(rw)
		go c.serve(ctx)
	}
}

func (c *conn) serve(ctx context.Context) {
	buf := bufio.NewReader(c.C)
	data := make([]byte, 1026) //1024 for the URL, 2 for the CRLF
	//req, overflow, err := req_buf.ReadLine()
	count := 0
	for count < 1026 {
		if strings.Contains(string(data), "\r\n") {
			break
		}
		b, err := buf.ReadByte()
		if err != nil {
			log.Printf("WARN: Couldn't serve request: %v", err.Error())
			c.C.Close()
			return
		}
		data[count] = b
		count = count + 1
	}
	var res Response
	var req string
	if !strings.Contains(string(data), "\r\n") {
		res = Response{STATUS_BAD_REQUEST, "Request too large", ""}
		req = "TOO_LONG_REQUEST"
	} else if !utf8.Valid(data) {
		res = Response{STATUS_BAD_REQUEST, "URL contains non UTF8 charcaters", ""}
	} else {
		req = string(data[:count-2])
		res = c.server.ParseRequest(req, c)
	}
	c.sendResponse(res)
	log.Printf("%v requested %v; responded with %v %v", c.C.RemoteAddr(), req, res.Status, res.Meta)
	c.C.Close()
}

func (s *Server) ParseRequest(req string, c *conn) Response {
	u, err := url.Parse(req)
	if err != nil {
		return Response{STATUS_BAD_REQUEST, "URL invalid", ""}
	}
	if u.Scheme == "" {
		u.Scheme = "gemini"
	} else if u.Scheme != "gemini" {
		return Response{STATUS_PROXY_REQUEST_REFUSED, "Proxying by Scheme not currently supported", ""}
	}
	if u.Port() != "1965" && u.Port() != "" {
		return Response{STATUS_PROXY_REQUEST_REFUSED, "Proxying by Port not currently supported", ""}
	}
	if u.Host == "" {
		return Response{STATUS_BAD_REQUEST, "Need to specify a host", ""}
	} else if s.HostnameToRoot[u.Hostname()] == "" {
		return Response{STATUS_PROXY_REQUEST_REFUSED, "Proxying by Hostname not currently supported", ""}
	}
	if strings.Contains(u.Path, "..") {
		return Response{STATUS_PERMANENT_FAILURE, "Dots in path, assuming bad faith.", ""}
	}

	selector := s.HostnameToRoot[u.Hostname()] + u.Path
	fi, err := os.Stat(selector)
	switch {
	case err != nil:
		// File doesn't exist.
		return Response{STATUS_NOT_FOUND, "Couldn't find file", ""}
	case os.IsNotExist(err) || os.IsPermission(err):
		return Response{STATUS_NOT_FOUND, "File does not exist", ""}
	case isNotWorldReadable(fi):
		return Response{STATUS_TEMPORARY_FAILURE, "Unable to access file", ""}
	case fi.IsDir():
		if strings.HasSuffix(u.Path, "/") {
			return generateDirectory(selector)
		} else {
			return Response{STATUS_REDIRECT_PERMANENT, "gemini://" + u.Hostname() + u.Path + "/", ""}
		}
	default:
		// it's a file
		matches, err := filepath.Glob(s.HostnameToCGI[u.Hostname()] + "/*")
		if err != nil {
			log.Printf("%v: Couldn't search for CGI: %v", u.Hostname(), err)
			return Response{STATUS_TEMPORARY_FAILURE, "Error finding file", ""}
		}
		if matches != nil && fi.Mode().Perm()&0111 == 0111 {
			//CGI file found
			return generateCGI(u, c)
		} else {
			//Normal file found
			return generateFile(selector)
		}
	}
}

func (c *conn) sendResponse(r Response) error {
	c.C.Write([]byte(fmt.Sprintf("%v %v\r\n", r.Status, r.Meta)))
	if r.Body != "" {
		c.C.Write([]byte(r.Body))
	}
	return nil
}

func isNotWorldReadable(file os.FileInfo) bool {
	return uint64(file.Mode().Perm())&0444 != 0444
}
