// ido-v3-set-review produces the exact bytes an independent authority reviews
// and signs, or verifies an externally produced Ed25519 signature. It does not
// generate keys, sign, connect to production, or enable any V3 runtime gate.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"joincare_server/pkg/common/constant"
	"joincare_server/pkg/common/idov3whitelist"
	"joincare_server/pkg/core/service/project"
)

const domain = "joincare:ido-v3:set:v1\x00"

func readStrict[T any](path string) (T, error) {
	var value T
	f, err := os.Open(path)
	if err != nil {
		return value, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil {
		return value, err
	}
	if len(data) > 64*1024 {
		return value, errors.New("input exceeds 64 KiB")
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&value); err != nil {
		return value, err
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return value, errors.New("unexpected trailing data")
	}
	return value, nil
}

func signingBytes(m idov3whitelist.SetManifest, req project.OnchainParamsRequest) ([]byte, error) {
	if m.ProjectID == 0 || m.ContractVersion != constant.ContractVersionIDOV3Stablecoin ||
		m.FundingMode != constant.FundingModeCappedPresale || req.ContractVersion != m.ContractVersion ||
		req.PaymentToken != m.PaymentSymbol || !strings.EqualFold(req.TokenAddress, m.ProjectToken) ||
		!strings.EqualFold(req.OwnerAddress, m.ProjectOwner) {
		return nil, errors.New("project/request identity mismatch")
	}
	request, err := json.Marshal(req) // exactly v3SetDigest in the Backend
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(request)
	if m.RequestDigest != common.BytesToHash(digest[:]).Hex() {
		return nil, errors.New("manifest request_digest differs from the exact typed request")
	}
	manifest, err := json.Marshal(m) // exactly VerifySignedSetManifest in the Backend
	if err != nil {
		return nil, err
	}
	return append([]byte(domain), manifest...), nil
}

func run(args []string) error {
	fs := flag.NewFlagSet("ido-v3-set-review", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "", "reviewed SetManifest JSON")
	requestPath := fs.String("request", "", "exact OnchainParamsRequest JSON")
	outputPath := fs.String("preimage", "", "output path for exact signing bytes; must not exist")
	publicPath := fs.String("public-key", "", "base64 Ed25519 public key file for verification")
	signaturePath := fs.String("signature", "", "base64 detached signature file for verification")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *manifestPath == "" || *requestPath == "" ||
		((*publicPath == "") != (*signaturePath == "")) ||
		(*outputPath == "" && *signaturePath == "") ||
		(*outputPath != "" && *signaturePath != "") {
		return errors.New("use --manifest and --request with exactly one of --preimage or --public-key plus --signature")
	}
	m, err := readStrict[idov3whitelist.SetManifest](*manifestPath)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	req, err := readStrict[project.OnchainParamsRequest](*requestPath)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	preimage, err := signingBytes(m, req)
	if err != nil {
		return err
	}
	if *outputPath != "" {
		f, err := os.OpenFile(*outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(preimage)
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			os.Remove(*outputPath)
			return errors.Join(writeErr, closeErr)
		}
		sum := sha256.Sum256(preimage)
		fmt.Printf("project=%d preimage_sha256=%x (not admission; no signature generated)\n", m.ProjectID, sum)
		return nil
	}
	pubText, err := os.ReadFile(*publicPath)
	if err != nil {
		return err
	}
	sigText, err := os.ReadFile(*signaturePath)
	if err != nil {
		return err
	}
	pub, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(string(pubText)))
	if err != nil {
		return errors.New("invalid public-key encoding")
	}
	sig, err := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(string(sigText)))
	if err != nil {
		return errors.New("invalid signature encoding")
	}
	if err := idov3whitelist.VerifySignedSetManifest(ed25519.PublicKey(pub), m, sig,
		common.HexToHash(m.RequestDigest), time.Now()); err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), preimage, sig) {
		return errors.New("signature differs from reviewed preimage")
	}
	sum := sha256.Sum256(preimage)
	fmt.Printf("verified project=%d preimage_sha256=%s public_key_sha256=%x\n", m.ProjectID, hex.EncodeToString(sum[:]), sha256.Sum256(pub))
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ido-v3-set-review:", err)
		os.Exit(1)
	}
}
