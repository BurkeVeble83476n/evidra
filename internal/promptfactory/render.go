package promptfactory

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	promptdata "samebits.com/evidra/prompts"
)

type renderData struct {
	ContractVersion string
	SkillVersion    string
	Contract        Contract
	Classification  Classification
	Output          OutputContracts
}

type renderSpec struct {
	id        string
	template  string
	generated string
	active    string
}

func RenderFiles(rootDir string, bundle Bundle) ([]RenderedFile, error) {
	specs := requiredRenderSpecs(bundle.Contract.Version)

	templateBase := filepath.Join(rootDir, "prompts", "source", "contracts", bundle.Contract.Version)
	specs = appendOptionalRenderSpec(specs, templateBase, renderSpec{
		id:        "mcp.prescribe",
		template:  "templates/mcp/prescribe.tmpl",
		generated: filepath.Join("prompts", "generated", bundle.Contract.Version, "mcpserver", "tools", "prescribe_description.txt"),
		active:    filepath.Join("prompts", "mcpserver", "tools", "prescribe_description.txt"),
	})
	specs = appendOptionalRenderSpec(specs, templateBase, renderSpec{
		id:        "mcp.prescribe_full",
		template:  "templates/mcp/prescribe_full.tmpl",
		generated: filepath.Join("prompts", "generated", bundle.Contract.Version, "mcpserver", "tools", "prescribe_full_description.txt"),
		active:    filepath.Join("prompts", "mcpserver", "tools", "prescribe_full_description.txt"),
	})
	specs = appendOptionalRenderSpec(specs, templateBase, renderSpec{
		id:        "mcp.prescribe_smart",
		template:  "templates/mcp/prescribe_smart.tmpl",
		generated: filepath.Join("prompts", "generated", bundle.Contract.Version, "mcpserver", "tools", "prescribe_smart_description.txt"),
		active:    filepath.Join("prompts", "mcpserver", "tools", "prescribe_smart_description.txt"),
	})
	specs = appendOptionalRenderSpec(specs, templateBase, renderSpec{
		id:        "skill.skill_smart",
		template:  "templates/skill/SKILL_SMART.tmpl",
		generated: filepath.Join("prompts", "generated", bundle.Contract.Version, "skill", "SKILL_SMART.md"),
		active:    filepath.Join("prompts", "skill", "SKILL_SMART.md"),
	})
	specs = appendOptionalRenderSpec(specs, templateBase, renderSpec{
		id:        "mcp.prompt_prescribe_smart",
		template:  "templates/mcp/prompt_prescribe_smart.tmpl",
		generated: filepath.Join("prompts", "generated", bundle.Contract.Version, "mcp", "prompt_prescribe_smart.md"),
		active:    filepath.Join("prompts", "mcp", "prompt_prescribe_smart.md"),
	})
	specs = appendOptionalRenderSpec(specs, templateBase, renderSpec{
		id:        "mcp.prompt_prescribe_full",
		template:  "templates/mcp/prompt_prescribe_full.tmpl",
		generated: filepath.Join("prompts", "generated", bundle.Contract.Version, "mcp", "prompt_prescribe_full.md"),
		active:    filepath.Join("prompts", "mcp", "prompt_prescribe_full.md"),
	})
	specs = appendOptionalRenderSpec(specs, templateBase, renderSpec{
		id:        "mcp.prompt_diagnosis",
		template:  "templates/mcp/prompt_diagnosis.tmpl",
		generated: filepath.Join("prompts", "generated", bundle.Contract.Version, "mcp", "prompt_diagnosis.md"),
		active:    filepath.Join("prompts", "mcp", "prompt_diagnosis.md"),
	})

	data := renderData{
		ContractVersion: bundle.Contract.Version,
		SkillVersion:    promptdata.ParseSkillVersionFromContractVersion(bundle.Contract.Version),
		Contract:        bundle.Contract,
		Classification:  bundle.Classification,
		Output:          bundle.Output,
	}

	out := make([]RenderedFile, 0, len(specs))
	for _, spec := range specs {
		tplPath := filepath.Join(templateBase, spec.template)
		tplBytes, err := os.ReadFile(tplPath)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", tplPath, err)
		}
		tpl, err := template.New(spec.id).Option("missingkey=error").Parse(string(tplBytes))
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", tplPath, err)
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("execute template %s: %w", tplPath, err)
		}

		content := normalizeRenderedContent(buf.String())
		out = append(out, RenderedFile{
			ID:          spec.id,
			TemplateRel: filepath.ToSlash(filepath.Join("prompts", "source", "contracts", bundle.Contract.Version, spec.template)),
			OutputRel:   filepath.ToSlash(spec.generated),
			ActiveRel:   filepath.ToSlash(spec.active),
			Content:     content,
		})
	}

	return out, nil
}

func requiredRenderSpecs(contractVersion string) []renderSpec {
	return []renderSpec{
		{
			id:        "mcp.initialize",
			template:  "templates/mcp/initialize.tmpl",
			generated: filepath.Join("prompts", "generated", contractVersion, "mcpserver", "initialize", "instructions.txt"),
			active:    filepath.Join("prompts", "mcpserver", "initialize", "instructions.txt"),
		},
		{
			id:        "mcp.report",
			template:  "templates/mcp/report.tmpl",
			generated: filepath.Join("prompts", "generated", contractVersion, "mcpserver", "tools", "report_description.txt"),
			active:    filepath.Join("prompts", "mcpserver", "tools", "report_description.txt"),
		},
		{
			id:        "mcp.get_event",
			template:  "templates/mcp/get_event.tmpl",
			generated: filepath.Join("prompts", "generated", contractVersion, "mcpserver", "tools", "get_event_description.txt"),
			active:    filepath.Join("prompts", "mcpserver", "tools", "get_event_description.txt"),
		},
		{
			id:        "mcp.agent_contract",
			template:  "templates/mcp/agent_contract.tmpl",
			generated: filepath.Join("prompts", "generated", contractVersion, "mcpserver", "resources", "content", "agent_contract_v1.md"),
			active:    filepath.Join("prompts", "mcpserver", "resources", "content", "agent_contract_v1.md"),
		},
		{
			id:        "runtime.system",
			template:  "templates/runtime/system_instructions.tmpl",
			generated: filepath.Join("prompts", "generated", contractVersion, "experiments", "runtime", "system_instructions.txt"),
			active:    filepath.Join("prompts", "experiments", "runtime", "system_instructions.txt"),
		},
		{
			id:        "runtime.agent_contract",
			template:  "templates/runtime/agent_contract.tmpl",
			generated: filepath.Join("prompts", "generated", contractVersion, "experiments", "runtime", "agent_contract_v1.md"),
			active:    filepath.Join("prompts", "experiments", "runtime", "agent_contract_v1.md"),
		},
		{
			id:        "skill.skill",
			template:  "templates/skill/SKILL.tmpl",
			generated: filepath.Join("prompts", "generated", contractVersion, "skill", "SKILL.md"),
			active:    filepath.Join("prompts", "skill", "SKILL.md"),
		},
	}
}

func appendOptionalRenderSpec(specs []renderSpec, templateBase string, spec renderSpec) []renderSpec {
	if _, err := os.Stat(filepath.Join(templateBase, spec.template)); err == nil {
		return append(specs, spec)
	}
	return specs
}

func normalizeRenderedContent(in string) string {
	in = strings.ReplaceAll(in, "\r\n", "\n")
	lines := strings.Split(in, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	out := strings.Join(lines, "\n")
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	return out + "\n"
}
