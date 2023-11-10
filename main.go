package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"github.com/huyouba1/ansible-agent/conf"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/huyouba1/ansible-agent/ansible"
)

var (
	flConfig = flag.String("c", "conf/defaults.toml", "Server configuration file")
)

func realMain() int {
	flag.Parse()

	config := conf.DefaultConfig()
	if *flConfig != "" {
		if _, err := toml.DecodeFile(*flConfig, config); err != nil {
			log.Println(err)
			return 1
		}
	}

	l, err := net.Listen("tcp", ":8700")
	if err != nil {
		log.Println(err)
		return 1
	}

	if config.SSL.Enabled {
		cert, err := tls.LoadX509KeyPair(config.SSL.Certificate, config.SSL.PrivateKey)
		if err != nil {
			log.Println(err)
			return 1
		}

		tlsConfig := tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    x509.NewCertPool(),
			ClientAuth:   tls.NoClientCert,
		}
		if config.SSL.ClientCA != "" {
			data, err := ioutil.ReadFile(config.SSL.ClientCA)
			if err != nil {
				log.Println(err)
				return 1
			}
			tlsConfig.ClientCAs.AppendCertsFromPEM(data)
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
		l = tls.NewListener(l, &tlsConfig)
	}

	if config.HttpAuth.Enabled {
		ansible.AuthToken = config.HttpAuth.Token
	}

	server := ansible.NewServer()
	if config.Ldap.Enabled {
		if err := server.ConfigureLDAP(&config.Ldap); err != nil {
			log.Println(err)
			return 1
		}
	}
	if err := server.Serve(l); err != nil {
		log.Println(err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(realMain())
}
