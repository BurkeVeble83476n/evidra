package main

import (
	"samebits.com/evidra/internal/assessment"
	"samebits.com/evidra/internal/score"
)

const (
	assessmentModePreview    = assessment.AssessmentModePreview
	assessmentModeSufficient = assessment.AssessmentModeSufficient
)

type assessmentBasis = assessment.Basis

type operationAssessment struct {
	RiskLevel        string           `json:"risk_level"`
	Score            float64          `json:"score"`
	ScoreBand        string           `json:"score_band"`
	ScoringProfileID string           `json:"scoring_profile_id"`
	SignalSummary    map[string]int   `json:"signal_summary"`
	Confidence       score.Confidence `json:"confidence"`
	Basis            assessmentBasis  `json:"basis"`
}

func buildOperationAssessmentWithProfile(evidencePath, sessionID, riskLevel string, profile score.Profile) (operationAssessment, error) {
	snapshot, err := assessment.BuildAtPathWithProfile(evidencePath, sessionID, profile)
	if err != nil {
		return operationAssessment{}, err
	}
	return assessmentFromSnapshot(snapshot, riskLevel), nil
}

func assessmentFromSnapshot(snapshot assessment.Snapshot, riskLevel string) operationAssessment {
	return operationAssessment{
		RiskLevel:        riskLevel,
		Score:            snapshot.Score,
		ScoreBand:        snapshot.ScoreBand,
		ScoringProfileID: snapshot.ScoringProfileID,
		SignalSummary:    snapshot.SignalSummary,
		Confidence:       snapshot.Confidence,
		Basis:            snapshot.Basis,
	}
}
