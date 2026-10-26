package http1

import "crypto/tls"

const keyFile = "tls/private_key.pem"
const certFile = "tls/public_certificate.pem"

func GetTLSConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}
