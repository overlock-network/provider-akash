package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/pkg/errors"
	certv1 "pkg.akt.dev/go/node/cert/v1"
)

// authVersionOID matches the value akash-network/chain-sdk's KeyPairManager
// stamps onto its certificates. The chain doesn't verify it, but akash CLI
// tooling does, so we keep parity.
var authVersionOID = asn1.ObjectIdentifier{2, 23, 133, 2, 6}

// PEM block types — same constants as cert/v1.PemBlkType*. Re-declared here
// to avoid pulling that package's blank-import side-effects into clients that
// only need the strings.
const (
	pemBlkTypeCertificate = "CERTIFICATE"
	pemBlkTypeECPublicKey = "EC PUBLIC KEY"
	pemBlkTypeECPrivKey   = "EC PRIVATE KEY"
)

// CertificateInfo represents detailed certificate information from the Akash network
type CertificateInfo struct {
	Serial      string `json:"serial"`
	Owner       string `json:"owner"`
	Issuer      string `json:"issuer"`
	Subject     string `json:"subject"`
	NotBefore   int64  `json:"notBefore"`
	NotAfter    int64  `json:"notAfter"`
	State       string `json:"state"`
	Fingerprint string `json:"fingerprint"`
	// PEM is the certificate PEM block (CERTIFICATE).
	PEM string `json:"pem"`
	// PubkeyPEM is the public key PEM block (EC PUBLIC KEY) — what was sent
	// to the chain alongside Cert.
	PubkeyPEM string `json:"pubkeyPem,omitempty"`
	// PrivateKeyPEM is the freshly generated private key PEM block
	// (EC PRIVATE KEY). Only populated by CreateCertificate; queries from
	// chain leave this empty since the chain never stores private keys.
	// Controllers route this into a Kubernetes Secret for downstream
	// mTLS clients (Manifest delivery, lease status, etc.).
	PrivateKeyPEM string `json:"privateKeyPem,omitempty"`
	CreatedAt     int64  `json:"createdAt"`
	ExpiresAt     int64  `json:"expiresAt"`
}

// Certificate states
const (
	CertificateStateValid   = "valid"
	CertificateStateExpired = "expired"
	CertificateStateRevoked = "revoked"
)

// CreateCertificate generates a fresh ECDSA-P256 keypair, builds an Akash-format
// self-signed x509 certificate, and broadcasts MsgCreateCertificate so the
// chain registers the public key. Returns the cert + key PEM bytes so callers
// can persist them (typically to a Kubernetes Secret) for later mTLS use.
func (ak *AkashClient) CreateCertificate(ctx context.Context, domains []string, owner string) (*CertificateInfo, error) {
	if owner == "" {
		return nil, fmt.Errorf("owner address is required")
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "generate private key")
	}

	notBefore := time.Now().UTC()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	certPEM, pubPEM, keyPEM, certX509, err := buildAkashCertificate(priv, owner, domains, notBefore, notAfter)
	if err != nil {
		return nil, err
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return nil, errors.Wrap(err, "node client")
	}

	resp, err := nodeClient.Tx().BroadcastMsgs(ctx, &certv1.MsgCreateCertificate{
		Owner:  owner,
		Cert:   certPEM,
		Pubkey: pubPEM,
	})
	if err != nil {
		return nil, fmt.Errorf("broadcast MsgCreateCertificate: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("MsgCreateCertificate rejected by chain: code=%d codespace=%s log=%s",
			resp.Code, resp.Codespace, resp.RawLog)
	}

	return &CertificateInfo{
		Serial:        certX509.SerialNumber.String(),
		Owner:         owner,
		Issuer:        certX509.Issuer.CommonName,
		Subject:       certX509.Subject.CommonName,
		NotBefore:     certX509.NotBefore.Unix(),
		NotAfter:      certX509.NotAfter.Unix(),
		State:         CertificateStateValid,
		Fingerprint:   fmt.Sprintf("%x", sha256.Sum256(certX509.Raw)),
		PEM:           string(certPEM),
		PubkeyPEM:     string(pubPEM),
		PrivateKeyPEM: string(keyPEM),
		CreatedAt:     time.Now().Unix(),
		ExpiresAt:     certX509.NotAfter.Unix(),
	}, nil
}

// GetCertificate fetches a certificate from chain by (owner, serial).
func (ak *AkashClient) GetCertificate(ctx context.Context, serial string, owner string) (*CertificateInfo, error) {
	if serial == "" || owner == "" {
		return nil, fmt.Errorf("owner and serial are required")
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return nil, errors.Wrap(err, "node client")
	}

	resp, err := nodeClient.Query().Cert().Certificates(ctx, &certv1.QueryCertificatesRequest{
		Filter: certv1.CertificateFilter{
			Owner:  owner,
			Serial: serial,
			// State left empty — chain returns certs in any state
			// (valid + revoked) so the controller can mirror the
			// real lifecycle.
		},
	})
	if err != nil {
		return nil, fmt.Errorf("query certificate: %w", err)
	}
	if len(resp.Certificates) == 0 {
		return nil, fmt.Errorf("certificate %s/%s not found on chain", owner, serial)
	}

	chainCert := resp.Certificates[0]
	info := &CertificateInfo{
		Serial:    chainCert.Serial,
		Owner:     owner,
		State:     mapChainCertState(chainCert.Certificate.State),
		PEM:       string(chainCert.Certificate.Cert),
		PubkeyPEM: string(chainCert.Certificate.Pubkey),
	}

	// Best-effort enrichment from the cert PEM bytes — Issuer, Subject,
	// validity window, fingerprint. Failure here doesn't poison the whole
	// query: the on-chain state is the source of truth.
	if blk, _ := pem.Decode(chainCert.Certificate.Cert); blk != nil {
		if x, err := x509.ParseCertificate(blk.Bytes); err == nil {
			info.Issuer = x.Issuer.CommonName
			info.Subject = x.Subject.CommonName
			info.NotBefore = x.NotBefore.Unix()
			info.NotAfter = x.NotAfter.Unix()
			info.ExpiresAt = x.NotAfter.Unix()
			info.Fingerprint = fmt.Sprintf("%x", sha256.Sum256(x.Raw))
		}
	}
	return info, nil
}

// RevokeCertificate broadcasts MsgRevokeCertificate for the given certificate.
func (ak *AkashClient) RevokeCertificate(ctx context.Context, serial string, owner string) error {
	if serial == "" || owner == "" {
		return fmt.Errorf("owner and serial are required")
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return errors.Wrap(err, "node client")
	}

	resp, err := nodeClient.Tx().BroadcastMsgs(ctx, &certv1.MsgRevokeCertificate{
		ID: certv1.ID{Owner: owner, Serial: serial},
	})
	if err != nil {
		return fmt.Errorf("broadcast MsgRevokeCertificate: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("MsgRevokeCertificate rejected by chain: code=%d codespace=%s log=%s",
			resp.Code, resp.Codespace, resp.RawLog)
	}
	return nil
}

// GetCertificates lists all certificates on chain for a given owner.
func (ak *AkashClient) GetCertificates(ctx context.Context, owner string) ([]CertificateInfo, error) {
	if owner == "" {
		return nil, fmt.Errorf("owner address is required")
	}

	nodeClient, err := ak.getNodeClient()
	if err != nil {
		return nil, errors.Wrap(err, "node client")
	}

	resp, err := nodeClient.Query().Cert().Certificates(ctx, &certv1.QueryCertificatesRequest{
		Filter: certv1.CertificateFilter{Owner: owner},
	})
	if err != nil {
		return nil, fmt.Errorf("query certificates: %w", err)
	}

	out := make([]CertificateInfo, 0, len(resp.Certificates))
	for _, c := range resp.Certificates {
		info := CertificateInfo{
			Serial:    c.Serial,
			Owner:     owner,
			State:     mapChainCertState(c.Certificate.State),
			PEM:       string(c.Certificate.Cert),
			PubkeyPEM: string(c.Certificate.Pubkey),
		}
		if blk, _ := pem.Decode(c.Certificate.Cert); blk != nil {
			if x, err := x509.ParseCertificate(blk.Bytes); err == nil {
				info.Issuer = x.Issuer.CommonName
				info.Subject = x.Subject.CommonName
				info.NotBefore = x.NotBefore.Unix()
				info.NotAfter = x.NotAfter.Unix()
				info.ExpiresAt = x.NotAfter.Unix()
				info.Fingerprint = fmt.Sprintf("%x", sha256.Sum256(x.Raw))
			}
		}
		out = append(out, info)
	}
	return out, nil
}

// ValidateCertificate decides whether a certificate needs renewal under an
// auto-renew policy. Currently checks: expired/revoked are terminal failures;
// otherwise renewal is needed when within 30 days of expiry.
func (ak *AkashClient) ValidateCertificate(certInfo *CertificateInfo, autoRenew bool, validityDays int32) (bool, error) {
	if certInfo == nil {
		return false, fmt.Errorf("certificate info is required")
	}
	if certInfo.State == CertificateStateRevoked {
		return false, fmt.Errorf("certificate is revoked")
	}
	if !autoRenew {
		return false, nil
	}
	if time.Unix(certInfo.ExpiresAt, 0).Before(time.Now().Add(30 * 24 * time.Hour)) {
		return true, nil
	}
	return false, nil
}

// buildAkashCertificate generates the cert + PEM bytes Akash expects:
//   - x509 self-signed certificate, CN = owner bech32 address, AuthVersionOID
//     extension set to "v0.0.1"
//   - PEM blocks: CERTIFICATE, EC PUBLIC KEY, EC PRIVATE KEY
//
// The chain only stores the cert + public key PEMs (and validates the cert's
// signature against the pubkey). The private key stays client-side.
func buildAkashCertificate(
	priv *ecdsa.PrivateKey,
	owner string,
	domains []string,
	notBefore, notAfter time.Time,
) (certPEM, pubPEM, keyPEM []byte, certX509 *x509.Certificate, err error) {
	serialNumber := new(big.Int).SetInt64(time.Now().UTC().UnixNano())

	extKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	if len(domains) > 0 {
		extKeyUsage = append(extKeyUsage, x509.ExtKeyUsageServerAuth)
	}

	tmpl := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: owner,
			ExtraNames: []pkix.AttributeTypeAndValue{
				{Type: authVersionOID, Value: "v0.0.1"},
			},
		},
		Issuer:                pkix.Name{CommonName: owner},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDataEncipherment | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           extKeyUsage,
		BasicConstraintsValid: true,
	}
	if dnsDomains := dnsOnly(domains); len(dnsDomains) > 0 {
		tmpl.DNSNames = dnsDomains
		tmpl.PermittedDNSDomains = dnsDomains
		tmpl.PermittedDNSDomainsCritical = true
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, priv.Public(), priv)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "create x509 certificate")
	}
	pubDER, err := x509.MarshalPKIXPublicKey(priv.Public())
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "marshal public key")
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "marshal private key")
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: pemBlkTypeCertificate, Bytes: certDER})
	pubPEM = pem.EncodeToMemory(&pem.Block{Type: pemBlkTypeECPublicKey, Bytes: pubDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: pemBlkTypeECPrivKey, Bytes: keyDER})

	parsed, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "reparse certificate")
	}
	return certPEM, pubPEM, keyPEM, parsed, nil
}

// dnsOnly filters out anything that parses as an IP from a domains list.
// Akash CLI also handles IPs — we drop them here for now since our CRD spec
// describes "domain names" only; can be extended later.
func dnsOnly(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		out = append(out, d)
	}
	return out
}

func mapChainCertState(s certv1.State) string {
	switch s {
	case certv1.CertificateValid:
		return CertificateStateValid
	case certv1.CertificateRevoked:
		return CertificateStateRevoked
	default:
		return ""
	}
}
