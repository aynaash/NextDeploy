package shared

// import (
// 	"crypto/tls"
// 	"crypto/x509"
// 	"errors"
// 	"time"
// )
//
// func VerifyCertificateChain(cert *x509.Certificate, roots *x509.CertPool) error {
// 	opts := x509.VerifyOptions{
// 		Roots:       roots,
// 		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
// 		CurrentTime: time.Now(),
// 	}
//
// 	if _, err := cert.Verify(opts); err != nil {
// 		return err
// 	}
//
// 	// Check certificate purpose flags
// 	if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
// 		return errors.New("certificate not valid for signing")
// 	}
//
// 	return nil
// }
//
// func TLSServerConfig(trustStore *TrustStore) *tls.Config {
// 	return &tls.Config{
// 		ClientAuth: tls.RequireAndVerifyClientCert,
// 		ClientCAs:  trustStore.CertPool(),
// 		MinVersion: tls.VersionTLS13,
// 	}
// }
