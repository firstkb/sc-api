//go:build !windows && !nacl && !plan9

package logging

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/phuslu/log"

	"github.com/firstkb/sc-api/internal/utils"
)

const (
	devNull = "/dev/null"

	defaultSyslogProtocol = "udp"
	defaultSyslogAddress  = "127.0.0.1:514"

	syslogError = "syslog initialization failed"

	logDescription = " <path>|syslog[=url]\vUse a custom log location or redirect to syslog"
)

func getDefaultLogFilePath() string {
	return filepath.Join("/var/log", utils.GetAppName())
}

// -------------------------------------------------------------------------------
// unix socket
// ------------------------
// syslog=unix:///tmp/syslog.sock		// SOCK_STREAM
// syslog=unixgram:///tmp/syslog.sock	// SOCK_DGRAM
// syslog=unixpacket:///tmp/syslog.sock	// SOCK_SEQPACKET
// ------------------------
// network address ip:port
// ------------------------
// syslog=udp://localhost:514
// syslog=tcp://localhost:514
// syslog=tcp+tls://localhost:6514?env=<tls_env_file>
//
//	  TLS environment variables:
//			PWA103API_TLS_KEY
//			PWA103API_TLS_CERT
//			PWA103API_TLS_CACERT
//
// -------------------------------------------------------------------------------
func getSyslogWriter(logOutput string) (log.Writer, error) {
	logOutput = strings.TrimSpace(strings.ReplaceAll(strings.TrimPrefix(strings.ToLower(logOutput), "syslog="), "\\", "/"))

	if logOutput == "" || strings.EqualFold("syslog", logOutput) {
		logOutput = fmt.Sprintf("%s://%s", defaultSyslogProtocol, defaultSyslogAddress)
	}

	u, err := url.Parse(logOutput)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", syslogError, err.Error())
	}

	network := strings.ToLower(u.Scheme)
	if network != "udp" && network != "tcp" && network != "tls" && network != "tcp+tls" && network != "unix" && network != "unixgram" && network != "unixpacket" {
		return nil, fmt.Errorf("%s: invalid or unsupported network protocol '%s'", syslogError, network)
	}

	isTLS := false

	if network == "tcp+tls" {
		isTLS = true
		network = "tcp"
	}

	address := u.Host
	if address == "" && (network == "unix" || network == "unixgram" || network == "unixpacket") {
		address = u.Path
	}

	if address == "" {
		return nil, fmt.Errorf("%s: invalid network address '%s'", syslogError, address)
	}

	if u.Port() == "" && (network == "udp" || network == "tcp") {
		if isTLS {
			address += ":6514"
		} else {
			address += ":514"
		}
	}

	if isTLS {
		return nil, fmt.Errorf("%s: TLS is not supported yet", syslogError)
	}

	// TODO: tls
	/*
		caCert, _ := ioutil.ReadFile("cacert.pem") // Load the CA server
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		cert, err := tls.LoadX509KeyPair("clientcert.pem", "clientkey.pem") // Load the certificate and the client private key
		if err != nil {
			log.Fatal(err)
		}
		tlsConf := &tls.Config{ // Creating a TLS configuration
			RootCAs:      caCertPool,
			Certificates: []tls.Certificate{cert},
		}
	*/

	// TODO: syslog
	/*
		 &log.SyslogWriter{
			Network: network,
			Address: address,
			Tag:     "",
			Marker:  "@cee:",
			Dial:    net.Dial,
		}

			Dial:    func(network, addr string) (net.Conn, error) {
						conn, err := tls.Dial(network, addr, tlsConf)
						return conn, err
					},

	*/

	return nil, errors.New("syslog is not implemented yet")
}

func getEventLogWriter() (log.Writer, error) {
	return nil, errors.New("eventlog is only supported on Windows platform")
}
