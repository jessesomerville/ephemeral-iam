package proxy

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"golang.org/x/mod/semver"

	"github.com/spf13/viper"

	"github.com/jessesomerville/ephemeral-iam/internal/appconfig"
	util "github.com/jessesomerville/ephemeral-iam/internal/eiamutil"
	errorsutil "github.com/jessesomerville/ephemeral-iam/internal/errors"
)

// GenerateCerts creates the self signed TLS certificate for the HTTPS proxy
func GenerateCerts() error {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: "Failed to generate RSA key pair",
			Err: err,
		}
	}

	notBefore := time.Now()
	notAfter := notBefore.AddDate(1, 0, 0)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: "Failed to generate random serial number limit for x509 cert",
			Err: err,
		}
	}

	template := x509.Certificate{
		Version:      1,
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:            []string{"US"},
			Locality:           []string{"Unknown"},
			Organization:       []string{"Unknown"},
			OrganizationalUnit: []string{"ephemeral-iam"},
			CommonName:         fmt.Sprintf("gcloud proxy CA %s", appconfig.Version),
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		return errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: "Failed to create x509 Cert",
			Err: err,
		}
	}

	if err := writeToFile(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}, "server.pem", 0o640); err != nil {
		return errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: "Failed to write server.pem file",
			Err: err,
		}
	}
	if err := writeToFile(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}, "server.key", 0o400); err != nil {
		return errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: "Failed to write server.key file",
			Err: err,
		}
	}

	return nil
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func writeToFile(data *pem.Block, filename string, perm os.FileMode) error {
	filepath := filepath.Join(appconfig.GetConfigDir(), filename)
	fd, err := os.Create(filepath)
	if err != nil {
		if os.IsPermission(err) {
			if err := os.Remove(filepath); err != nil {
				return errorsutil.EiamError{
					Log: util.Logger.WithError(err),
					Msg: fmt.Sprintf("Failed to update %s", filepath),
					Err: err,
				}
			}
			return writeToFile(data, filename, perm)
		}
		return errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: fmt.Sprintf("Failed to write file %s", filepath),
			Err: err,
		}
	}

	defer fd.Close()

	if err := pem.Encode(fd, data); err != nil {
		return errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: "Failed to write PEM encoding to a file",
			Err: err,
		}
	}

	if err := os.Chmod(filepath, perm); err != nil {
		return errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: fmt.Sprintf("Failed to update the file permissions for %s", filepath),
			Err: err,
		}
	}
	return nil
}

func checkProxyCertificate() error {
	certFile := viper.GetString("authproxy.certfile")
	keyFile := viper.GetString("authproxy.keyfile")
	if certFile == "" || keyFile == "" {
		if keyFile == "" {
			util.Logger.Debug("Setting authproxy.keyfile")
			viper.Set("authproxy.keyfile", filepath.Join(appconfig.GetConfigDir(), "server.key"))
			keyFile = viper.GetString("authproxy.keyfile")
		}
		if certFile == "" {
			util.Logger.Debug("Setting authproxy.certfile")
			viper.Set("authproxy.certfile", filepath.Join(appconfig.GetConfigDir(), "server.pem"))
			certFile = viper.GetString("authproxy.certfile")
		}
		if err := viper.WriteConfig(); err != nil {
			return errorsutil.EiamError{
				Log: util.Logger.WithError(err),
				Msg: "Failed to write configuration file",
				Err: err,
			}
		}
	}

	_, certErr := os.Stat(certFile)
	_, keyErr := os.Stat(keyFile)
	certExists := !os.IsNotExist(certErr)
	keyExists := !os.IsNotExist(keyErr)

	if certExists != keyExists { // Check if only one of either the key or the cert exist
		util.Logger.Warn("Either the auth proxy cert or key is missing. Regenerating both")
		return GenerateCerts()
	} else if !certExists { // Check if neither files exist
		util.Logger.Debug("Generating proxy cert and key files")
		return GenerateCerts()
	}

	return validateProxyCert(certFile)
}

func validateProxyCert(certFile string) (err error) {
	cert, err := readCert(certFile)
	if err != nil {
		return err
	}

	// Check if certificate version matches eiam version
	certCN := cert.Subject.CommonName
	commonNamePattern := regexp.MustCompile(`^gcloud proxy CA (v[\d]+\.[\d]+\.[\d]+)$`)
	if !commonNamePattern.MatchString(certCN) {
		util.Logger.Warnf("Regenerating certs due to unrecognized CN field: %s", certCN)
		return GenerateCerts()
	}
	var certSemVer string
	if semverGrp := commonNamePattern.FindStringSubmatch(certCN); len(semverGrp) > 0 {
		certSemVer = semverGrp[0]
	} else {
		util.Logger.Warnf("Regenerating cert due to invalid cert common name: %s", certCN)
		return GenerateCerts()
	}
	if semver.Compare(appconfig.Version, certSemVer) > 0 {
		util.Logger.Debug("Certificate is outdated, generating new one")
		return GenerateCerts()
	}

	if !cert.IsCA {
		util.Logger.Warn("Regenerating cert due to invalid existing certificate options")
		return GenerateCerts()
	}
	return nil
}

func readCert(certFile string) (cert *x509.Certificate, err error) {
	var certBytes []byte
	var certBlock *pem.Block

	if certBytes, err = ioutil.ReadFile(certFile); err != nil {
		return nil, errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: "Failed to read certificate file",
			Err: err,
		}
	}
	if certBlock, _ = pem.Decode(certBytes); certBlock == nil {
		return nil, errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: "Failed to decode certificate bytes",
			Err: err,
		}
	}
	if cert, err = x509.ParseCertificate(certBlock.Bytes); err != nil {
		return nil, errorsutil.EiamError{
			Log: util.Logger.WithError(err),
			Msg: "Failed to parse certificate bytes",
			Err: err,
		}
	}
	return cert, nil
}
