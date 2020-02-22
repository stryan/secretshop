package main

import "fmt"

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
