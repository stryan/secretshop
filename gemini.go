package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Yoinked from jetforce and go'ified
const (
	STATUS_INPUT = 10

	STATUS_SUCCESS                = 20
	STATUS_SUCCESS_END_OF_SESSION = 21

	STATUS_REDIRECT_TEMPORARY = 30
	STATUS_REDIRECT_PERMANENT = 31

	STATUS_TEMPORARY_FAILURE  = 40
	STATUS_SERVER_UNAVAILABLE = 41
	STATUS_CGI_ERROR          = 42
	STATUS_PROXY_ERROR        = 43
	STATUS_SLOW_DOWN          = 44

	STATUS_PERMANENT_FAILURE     = 50
	STATUS_NOT_FOUND             = 51
	STATUS_GONE                  = 52
	STATUS_PROXY_REQUEST_REFUSED = 53
	STATUS_BAD_REQUEST           = 59

	STATUS_CLIENT_CERTIFICATE_REQUIRED     = 60
	STATUS_TRANSIENT_CERTIFICATE_REQUESTED = 61
	STATUS_AUTHORISED_CERTIFICATE_REQUIRED = 62
	STATUS_CERTIFICATE_NOT_ACCEPTED        = 63
	STATUS_FUTURE_CERTIFICATE_REJECTED     = 64
	STATUS_EXPIRED_CERTIFICATE_REJECTED    = 65
)

type Response struct {
	Status int
	Meta   string
	Body   string
}

type GeminiConfig struct {
	Hostname string
	KeyFile  string
	CertFile string
	RootDir  string
	CGIDir   string
}

func (c *GeminiConfig) String() string {
	return fmt.Sprintf("Gemini Config: %v Files:%v CGI:%v", c.Hostname, c.RootDir, c.CGIDir)
}

func generateFile(selector string) Response {
	meta := mime.TypeByExtension(filepath.Ext(selector))
	if meta == "" {
		//assume plain UTF-8 text
		meta = "text/gemini; charset=utf-8"
	}
	file, err := os.Open(selector)
	if err != nil {
		panic("Failed to read file")
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	return Response{STATUS_SUCCESS, meta, string(buf)}
}

func generateDirectory(path string) Response {
	var dirpage string
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Println(err)
		return Response{STATUS_TEMPORARY_FAILURE, "Unable to show directory dirpage", ""}
	}
	dirpage = "# Directory Contents\r\n"
	for _, file := range files {
		// Don't list hidden files
		if isNotWorldReadable(file) || strings.HasPrefix(file.Name(), ".") {
			continue
		}
		if file.Name() == "index.gmi" || file.Name() == "index.gemini" {
			//Found an index file, return that instead
			return generateFile(path + file.Name())
		} else {
			dirpage += fmt.Sprintf("=> %s %s\r\n", file.Name(), file.Name())
		}
	}
	return Response{STATUS_SUCCESS, "text/gemini", dirpage}
}

func generateCGI(selector *url.URL, c *conn) Response {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second) //make this customizable
	defer cancel()

	cmd := exec.CommandContext(ctx, selector.Path)

	//build CGI environment
	cmd.Env = []string{fmt.Sprintf("GEMINI_URL=%v", selector),
		fmt.Sprintf("HOSTNAME=%v", selector.Hostname()),
		fmt.Sprintf("PATH_INFO=%v", selector.Path),
		fmt.Sprintf("QUERY_STRING=%v", selector.RawQuery),
		fmt.Sprintf("REMOTE_ADDR=%v", c.C.RemoteAddr()),
		fmt.Sprintf("REMOTE_HOST=%v", c.C.RemoteAddr()),
		fmt.Sprintf("SERVER_NAME=%v", selector.Hostname()),
		fmt.Sprintf("SERVER_PORT=%v", c.server.Port),
		"SERVER_PROTOCOL=GEMINI",
		"SERVER_SOFTWARE=secretshop/0.1.0",
	}
	cmdout, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("CGI %v timed out", selector)
		return Response{STATUS_CGI_ERROR, "CGI process timed out", ""}
	}
	if err != nil {
		log.Printf("Error running CGI process %v", selector)
		return Response{STATUS_CGI_ERROR, "Error running CGI process", ""}
	}

	header, _, err := bufio.NewReader(strings.NewReader(string(cmdout))).ReadLine()
	if err != nil {
		log.Printf("Error running CGI process %v", selector)
		return Response{STATUS_CGI_ERROR, "Error running CGI process", ""}
	}
	header_s := strings.Fields(string(header))
	//make sure it has a valid status
	status, err := strconv.Atoi(header_s[0])
	if err != nil {
		log.Printf("Error running CGI process %v", selector)
		return Response{STATUS_CGI_ERROR, "Error running CGI process", ""}
	}
	if status < 0 || status > 69 {
		log.Printf("CGI script returned bad status %v", selector)
		return Response{STATUS_CGI_ERROR, "Error running CGI process", ""}
	}
	return Response{status, "", string(cmdout)}
}
