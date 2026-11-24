package idov3whitelist

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// This optional, local-only test compares the compiled *contract* artifact to
// the reader's assumptions. Set JOINCARE_IDO_V3_ARTIFACT to the local
// JoinCareIDOv3.json path explicitly; ordinary Backend tests need no sibling
// repository or compiled artifact.
func TestV3ReaderMatchesCompiledContractABI(t *testing.T) {
	path := os.Getenv("JOINCARE_IDO_V3_ARTIFACT")
	if path == "" {
		t.Skip("compiled V3 contract artifact not supplied")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("V3 contract artifact unavailable")
	}
	if err := checkV3ReaderArtifact(raw); err != nil {
		t.Fatalf("V3 contract ABI changed: %v", err)
	}
}

func checkV3ReaderArtifact(raw []byte) error {
	var artifact struct {
		ContractName string          `json:"contractName"`
		ABI          json.RawMessage `json:"abi"`
	}
	if json.Unmarshal(raw, &artifact) != nil || artifact.ContractName != "JoinCareIDOv3" || len(artifact.ABI) == 0 {
		return errors.New("wrong or malformed V3 artifact")
	}
	parsed, err := abi.JSON(bytes.NewReader(artifact.ABI))
	if err != nil {
		return errors.New("V3 artifact ABI invalid")
	}
	check := func(name string, input, output []string, mutability string) error {
		method, ok := parsed.Methods[name]
		if !ok || method.StateMutability != mutability {
			return errors.New("V3 method missing or mutability changed: " + name)
		}
		inputs, outputs := make([]string, 0, len(method.Inputs)), make([]string, 0, len(method.Outputs))
		for _, v := range method.Inputs {
			inputs = append(inputs, v.Type.String())
		}
		for _, v := range method.Outputs {
			outputs = append(outputs, v.Type.String())
		}
		if !reflect.DeepEqual(inputs, input) || !reflect.DeepEqual(outputs, output) {
			return errors.New("V3 method types changed: " + name)
		}
		return nil
	}
	for _, spec := range []struct {
		name          string
		input, output []string
		mutability    string
	}{
		{"getProject", []string{"uint256"}, []string{"(uint256,address,address,address,address,uint8,uint8,uint16,uint8,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,bool,bool,bool,uint256[],uint256[])"}, "view"},
		{"getWhitelistPolicy", []string{"uint256"}, []string{"(bool,bytes32,uint64)"}, "view"},
		{"owner", []string{}, []string{"address"}, "view"},
		{"whitelistGovernor", []string{}, []string{"address"}, "view"},
		{"contractVersion", []string{}, []string{"bytes32"}, "pure"},
		{"purchaseWithProof", []string{"uint256", "uint256", "uint64", "bytes32[]"}, []string{}, "nonpayable"},
	} {
		if err := check(spec.name, spec.input, spec.output, spec.mutability); err != nil {
			return err
		}
	}
	project := parsed.Methods["getProject"]
	if len(project.Outputs) != 1 || len(project.Outputs[0].Type.TupleElems) != v3ProjectWords ||
		!reflect.DeepEqual(project.Outputs[0].Type.TupleRawNames, []string{
			"projectId", "projectToken", "paymentToken", "projectOwner", "feeReceiver", "projectTokenDecimals",
			"paymentTokenDecimals", "successFeeRate", "status", "tokenPrice", "hardCap", "softCap",
			"maxPerUser", "startTime", "endTime", "totalRaised", "totalTokensSold", "totalTokensDeposited",
			"fundsWithdrawn", "platformFeeWithdrawn", "projectTokensWithdrawn", "vestingTimes", "vestingPercents",
		}) {
		return errors.New("V3 project tuple fields changed")
	}
	return nil
}

func TestV3ReaderArtifactDriftFailsClosed(t *testing.T) {
	path := os.Getenv("JOINCARE_IDO_V3_ARTIFACT")
	if path == "" {
		t.Skip("compiled V3 contract artifact not supplied")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("V3 contract artifact unavailable")
	}
	var artifact map[string]json.RawMessage
	if json.Unmarshal(raw, &artifact) != nil {
		t.Fatal("malformed compiled artifact")
	}
	var entries []map[string]any
	if json.Unmarshal(artifact["abi"], &entries) != nil {
		t.Fatal("malformed compiled ABI")
	}
	mutate := func(change func([]map[string]any)) []byte {
		var clone []map[string]any
		encoded, _ := json.Marshal(entries)
		if json.Unmarshal(encoded, &clone) != nil {
			t.Fatal("cannot clone ABI")
		}
		change(clone)
		out, _ := json.Marshal(clone)
		wrapped, _ := json.Marshal(map[string]json.RawMessage{"contractName": artifact["contractName"], "abi": out})
		return wrapped
	}
	for name, change := range map[string]func([]map[string]any){
		"tuple field order": func(items []map[string]any) {
			for _, item := range items {
				if item["name"] == "getProject" {
					fields := item["outputs"].([]any)[0].(map[string]any)["components"].([]any)
					fields[1], fields[2] = fields[2], fields[1]
					return
				}
			}
		},
		"proof version type": func(items []map[string]any) {
			for _, item := range items {
				if item["name"] == "purchaseWithProof" {
					item["inputs"].([]any)[2].(map[string]any)["type"] = "uint256"
					return
				}
			}
		},
		"governor getter removed": func(items []map[string]any) {
			for _, item := range items {
				if item["name"] == "whitelistGovernor" {
					item["name"] = "changedGovernor"
					return
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := checkV3ReaderArtifact(mutate(change)); err == nil {
				t.Fatal("contract ABI drift accepted")
			}
		})
	}
}
