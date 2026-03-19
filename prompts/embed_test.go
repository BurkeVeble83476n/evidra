package prompts

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSkill_ReturnsNonEmptyContent(t *testing.T) {
	t.Parallel()

	content, err := ReadSkill()
	if err != nil {
		t.Fatalf("ReadSkill: %v", err)
	}
	if content == "" {
		t.Fatal("ReadSkill returned empty content")
	}
	if !strings.HasPrefix(content, "---\nname: evidra\n") {
		t.Fatal("skill missing YAML frontmatter")
	}
	if !strings.Contains(content, "prescribe") {
		t.Fatal("skill missing prescribe content")
	}
}

func TestReadSkill_IncludesSplitPrescribeGuidance(t *testing.T) {
	t.Parallel()

	content, err := ReadSkill()
	if err != nil {
		t.Fatalf("ReadSkill: %v", err)
	}
	if !strings.Contains(content, "`prescribe_smart`") {
		t.Fatalf("skill missing prescribe_smart guidance: %s", content)
	}
	if !strings.Contains(content, "`prescribe_full`") {
		t.Fatalf("skill missing prescribe_full guidance: %s", content)
	}
}

func TestReadMCPPrescribeFullDescription_IsArtifactSpecific(t *testing.T) {
	t.Parallel()

	content, err := Read(MCPPrescribeFullDescriptionPath)
	if err != nil {
		t.Fatalf("Read(%q): %v", MCPPrescribeFullDescriptionPath, err)
	}
	if !strings.Contains(content, "raw_artifact") {
		t.Fatalf("prescribe_full description missing raw_artifact guidance: %s", content)
	}
	if strings.Contains(content, "prescribe_smart") {
		t.Fatalf("prescribe_full description should stay tool-specific: %s", content)
	}
}

func TestReadMCPPrescribeSmartDescription_IsLightweight(t *testing.T) {
	t.Parallel()

	content, err := Read(MCPPrescribeSmartDescriptionPath)
	if err != nil {
		t.Fatalf("Read(%q): %v", MCPPrescribeSmartDescriptionPath, err)
	}
	if !strings.Contains(content, "resource") {
		t.Fatalf("prescribe_smart description missing resource guidance: %s", content)
	}
	if strings.Contains(content, "raw_artifact") {
		t.Fatalf("prescribe_smart description should not require raw_artifact: %s", content)
	}
}

func TestLegacyPrescribeDescription_IsNotEmbedded(t *testing.T) {
	t.Parallel()

	if _, err := Read("mcpserver/tools/prescribe_description.txt"); err == nil {
		t.Fatal("legacy prescribe description should not be embedded")
	}
}

func TestParseContractVersionHeader(t *testing.T) {
	t.Parallel()

	contract, ok := parseContractVersionHeader("# contract: v1.0\nHello")
	if !ok {
		t.Fatalf("expected header parse success")
	}
	if contract != "v1.0" {
		t.Fatalf("contract=%q, want v1.0", contract)
	}
}

func TestSkillVersionFromContractVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "v1.0", want: "1.0.0"},
		{in: "1.1", want: "1.1.0"},
		{in: "v1.2.3", want: "1.2.3"},
		{in: "", want: DefaultContractSkillVersion},
		{in: "garbage", want: DefaultContractSkillVersion},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := skillVersionFromContractVersion(tc.in); got != tc.want {
				t.Fatalf("skillVersionFromContractVersion(%q)=%q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStripContractHeader(t *testing.T) {
	t.Parallel()

	in := "# contract: v1.0\nline1\nline2\n"
	want := "line1\nline2"
	if got := stripContractHeader(in); got != want {
		t.Fatalf("stripContractHeader()=%q, want %q", got, want)
	}
}

func TestResolvePromptMetadata_RuntimeContract(t *testing.T) {
	t.Parallel()

	absPath, err := filepath.Abs("experiments/runtime/agent_contract_v1.md")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}

	meta, err := ResolvePromptMetadata(absPath)
	if err != nil {
		t.Fatalf("ResolvePromptMetadata: %v", err)
	}
	if meta.ContractVersion != DefaultContractVersion {
		t.Fatalf("contract_version = %q, want %q", meta.ContractVersion, DefaultContractVersion)
	}
	if meta.SkillVersion != DefaultContractSkillVersion {
		t.Fatalf("skill_version = %q, want %q", meta.SkillVersion, DefaultContractSkillVersion)
	}
	if meta.Path != "prompts/experiments/runtime/agent_contract_v1.md" {
		t.Fatalf("path = %q, want %q", meta.Path, "prompts/experiments/runtime/agent_contract_v1.md")
	}
	if meta.PromptVersion != "sha256:6d94c115a8d5c5641be5be89a526f3b27f7a54f9fdd5b8e96f16905696dc100e" {
		t.Fatalf("prompt_version = %q", meta.PromptVersion)
	}
}
