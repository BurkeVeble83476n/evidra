package promptfactory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

func LoadBundle(rootDir, contractVersion string) (Bundle, error) {
	if rootDir == "" {
		return Bundle{}, errors.New("rootDir is required")
	}
	if contractVersion == "" {
		return Bundle{}, errors.New("contractVersion is required")
	}

	base := filepath.Join(rootDir, "prompts", "source", "contracts", contractVersion)

	contractPath := filepath.Join(base, "CONTRACT.yaml")
	classificationPath := filepath.Join(base, "CLASSIFICATION.yaml")
	outputPath := filepath.Join(base, "OUTPUT_CONTRACTS.yaml")

	contractBytes, err := os.ReadFile(contractPath)
	if err != nil {
		return Bundle{}, fmt.Errorf("read contract yaml: %w", err)
	}
	classBytes, err := os.ReadFile(classificationPath)
	if err != nil {
		return Bundle{}, fmt.Errorf("read classification yaml: %w", err)
	}
	outputBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return Bundle{}, fmt.Errorf("read output contracts yaml: %w", err)
	}

	var contract Contract
	if err := yaml.Unmarshal(contractBytes, &contract); err != nil {
		return Bundle{}, fmt.Errorf("parse contract yaml: %w", err)
	}
	var classification Classification
	if err := yaml.Unmarshal(classBytes, &classification); err != nil {
		return Bundle{}, fmt.Errorf("parse classification yaml: %w", err)
	}
	var outputContracts OutputContracts
	if err := yaml.Unmarshal(outputBytes, &outputContracts); err != nil {
		return Bundle{}, fmt.Errorf("parse output contracts yaml: %w", err)
	}

	bundle := Bundle{
		Contract:       contract,
		Classification: classification,
		Output:         outputContracts,
	}
	if err := ValidateBundle(bundle, contractVersion); err != nil {
		return Bundle{}, err
	}

	return bundle, nil
}
