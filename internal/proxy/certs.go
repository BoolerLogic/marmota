package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

var CADirectory = getCADirectory()
var CACertPath = filepath.Join(CADirectory, "ca.crt")
var CAKeyPath = filepath.Join(CADirectory, "ca.key")

func keyID(pub *rsa.PublicKey) []byte {
	b := x509.MarshalPKCS1PublicKey(pub)
	h := sha1.Sum(b)
	return h[:]
}

func getCADirectory() string {
	// Obtiene la ruta base según el sistema operativo
	configDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}

	// windows: C:\Users\<Usuario>\AppData\Roaming\marmota\
	// mac: /Users/<Usuario>/Library/Application Support/marmota/
	// linux: /home/<Usuario>/.config/marmota/

	appDir := filepath.Join(configDir, "marmota")

	// Crea el directorio si no existe (con permisos de lectura/escritura/ejecución para el usuario)
	err = os.MkdirAll(appDir, 0700)
	if err != nil {
		panic(err)
	}

	return appDir
}

// Aqui creamos un Certificado que actua como Autoridad Certificadora (CA)
func genRootCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128) // 2^128
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "Marmota"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // Válido por 10 años
		IsCA:                  true,
		MaxPathLenZero:        true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		SubjectKeyId:          keyID(&caPrivKey.PublicKey),
		AuthorityKeyId:        keyID(&caPrivKey.PublicKey),
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	caCert, err := x509.ParseCertificate(derBytes)
	return caCert, caPrivKey, err
}

// Aqui creamos un certificado con un dominio especifico y firmado por una Autoridad Certificadora (CA)
func GenFakeCertSignedByCA(domain string, caCert *x509.Certificate, caPrivKey *rsa.PrivateKey) (tls.Certificate, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128) // 2^128
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: domain},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		SubjectKeyId:          keyID(&privKey.PublicKey),
		AuthorityKeyId:        keyID(&caPrivKey.PublicKey),
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// #8 Si el Servidor Destino fuese una IP en vez de un Dominio:
	if ip := net.ParseIP(domain); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{domain}
	}

	// fmt.Printf(" >>>>>>>>>>> Domain: %s\n", domain)

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &privKey.PublicKey, caPrivKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{
		// Certificate: [][]byte{derBytes, caCert.Raw},
		Certificate: [][]byte{derBytes},
		PrivateKey:  privKey,
	}, nil
}

func loadCACert(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

func loadCAKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func saveCACert(cert *x509.Certificate, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) // Permisos para que solo el usuario pueda leer la clave
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

func saveCAKey(key *rsa.PrivateKey, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) // Permisos para que solo el usuario pueda leer la clave
	if err != nil {
		return err
	}
	defer file.Close()

	keyBytes := x509.MarshalPKCS1PrivateKey(key)

	return pem.Encode(file, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	})
}

func GetOrCreateCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	// Intentar cargar
	caCert, err1 := loadCACert(CACertPath)
	caKey, err2 := loadCAKey(CAKeyPath)

	if err1 == nil && err2 == nil {
		pubKey, ok := caCert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, nil, errors.New("the loaded CA certificate does not use RSA")
		}
		if pubKey.N.Cmp(caKey.PublicKey.N) != 0 {
			return nil, nil, errors.New("the CA private key and CA certificate do not match")
		}

		return caCert, caKey, nil
	}

	// Si falla → generar
	log.Println("Generating new root certificates")
	cert, key, err := genRootCA()
	if err != nil {
		return nil, nil, err
	}

	// Guardar
	err3 := saveCACert(cert, CACertPath)
	err4 := saveCAKey(key, CAKeyPath)

	if err3 != nil {
		return cert, key, fmt.Errorf("error saving CA certificate: %v", err3)
	}

	if err4 != nil {
		return cert, key, fmt.Errorf("error saving CA private key: %v", err4)
	}

	return cert, key, nil
}

func RemoveCAFiles() error {
	paths := []string{
		CACertPath,
		CAKeyPath,
	}

	for _, path := range paths {
		err := os.Remove(path)

		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return fmt.Errorf("error remove ca files %s: %w", path, err)
		}
	}

	return nil
}
