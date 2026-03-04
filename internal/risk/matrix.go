package risk

// riskMatrix maps operationClass x scopeClass (environment-based) to riskLevel.
// See EVIDRA_AGENT_RELIABILITY_BENCHMARK.md section 7.
var riskMatrix = map[string]map[string]string{
	"read":    {"production": "low", "staging": "low", "development": "low", "unknown": "low"},
	"mutate":  {"production": "high", "staging": "medium", "development": "low", "unknown": "medium"},
	"destroy": {"production": "critical", "staging": "high", "development": "medium", "unknown": "high"},
	"plan":    {"production": "low", "staging": "low", "development": "low", "unknown": "low"},
}

// RiskLevel returns the risk level for the given operation and scope classes.
// Unknown combinations default to "high".
func RiskLevel(operationClass, scopeClass string) string {
	row, ok := riskMatrix[operationClass]
	if !ok {
		return "high"
	}
	level, ok := row[scopeClass]
	if !ok {
		return "high"
	}
	return level
}
