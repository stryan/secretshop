package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io/ioutil"
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
	// Gopher handlers with context.WithValue to access the address
	// the local address the connection arrived on.
	// The associated value will be of type net.Addr.
	LocalAddrContextKey = &contextKey{"local-addr"}
)

type Server struct {
	Addr string // TCP address to listen on, ":gemini" if empty

	Hostname   string // FQDN Hostname to reach this server on
	ServerRoot string //Root folder for gemini files
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
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	addr := s.Addr
	mime.AddExtensionType(".gmi", "text/gemini")
	mime.AddExtensionType(".gemini", "text/gemini")
	if addr == "" {
		addr = ":1965"
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("server: loadkeys: %s", err)
	}
	config := tls.Config{Certificates: []tls.Certificate{cert}}
	config.Rand = rand.Reader

	ln, err := tls.Listen("tcp", addr, &config)
	if err != nil {
		log.Fatalf("server: listen: %s", err)
	}

	return s.Serve(ln)
}
func ListenAndServeTLS(cp Config) error {
	server := &Server{Addr: ":" + cp.Port, ServerRoot: cp.RootDir, Hostname: cp.Hostname}
	return server.ListenAndServeTLS(cp.CertFile, cp.KeyFile)
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
	if !strings.Contains(string(data), "\r\n") {
		c.sendResponse(Response{STATUS_BAD_REQUEST, "Request too large", ""})
		c.C.Close()
		return
	}
	if !utf8.Valid(data) {
		c.sendResponse(Response{STATUS_BAD_REQUEST, "URL contains non UTF8 charcaters", ""})
		c.C.Close()
		return
	}
	req := string(data[:count-2])
	res := c.server.ParseRequest(req)
	c.sendResponse(res)
	c.C.Close()
}

func (s *Server) ParseRequest(req string) Response {
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
	} else if u.Hostname() != s.Hostname {
		return Response{STATUS_PROXY_REQUEST_REFUSED, "Proxying by Hostname not currently supported", ""}
	}
	if strings.Contains(u.Path, "..") {
		return Response{STATUS_PERMANENT_FAILURE, "Dots in path, assuming bad faith.", ""}
	}

	selector := s.ServerRoot + u.Path
	fi, err := os.Stat(selector)
	switch {
	case err != nil:
		// File doesn't exist.
		return Response{STATUS_NOT_FOUND, "Couldn't find file", ""}
	case os.IsNotExist(err) || os.IsPermission(err):
		return Response{STATUS_NOT_FOUND, "File does not exist", ""}
	case uint64(fi.Mode().Perm())&0444 != 0444:
		return Response{STATUS_TEMPORARY_FAILURE, "Unable to access file", ""}
	case fi.IsDir():
		if strings.HasSuffix(u.Path, "/") {
			return generateDirectory(selector)
		} else {
			return Response{STATUS_REDIRECT_PERMANENT, "gemini://" + s.Hostname + u.Path + "/", ""}
		}
	default:
		// it's a file
		return generateFile(selector)
	}
}

func generateFile(selector string) Response {
	meta := mime.TypeByExtension(filepath.Ext(selector))
	file, err := os.Open(selector)
	if err != nil {
		panic("Failed to read file")
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	return Response{STATUS_SUCCESS, meta, string(buf)}
}

func generateDirectory(path string) Response {
	var listing string
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Println(err)
		return Response{STATUS_TEMPORARY_FAILURE, "Unable to show directory listing", ""}
	}
	listing = "# Directory listing\r\n"
	for _, file := range files {
		// Skip dotfiles
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}
		// Only list world readable files
		if uint64(file.Mode().Perm())&0444 != 0444 {
			continue
		}
		if file.Name() == "index.gmi" || file.Name() == "index.gemini" {
			//Found an index file, return that instead
			return generateFile(path + file.Name())
		} else {
			listing += fmt.Sprintf("=> %s %s\r\n", file.Name(), file.Name())
		}
	}
	return Response{STATUS_SUCCESS, "text/gemini", listing}
}

func (c *conn) sendResponse(r Response) error {
	c.C.Write([]byte(fmt.Sprintf("%v %v\r\n", r.Status, r.Meta)))
	if r.Body != "" {
		c.C.Write([]byte(r.Body))
	}
	return nil
}
