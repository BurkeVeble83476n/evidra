package signal

import (
	"testing"
	"time"
)

func TestDetectRepairLoop(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []Entry{
		{EventID: "p1", IsPrescription: true, ActorID: "a", IntentDigest: "i1", ArtifactDigest: "sha256:a", Timestamp: now},
		{EventID: "r1", IsReport: true, PrescriptionID: "p1", ExitCode: intPtr(1), Timestamp: now.Add(10 * time.Second)},
		{EventID: "p2", IsPrescription: true, ActorID: "a", IntentDigest: "i1", ArtifactDigest: "sha256:b", Timestamp: now.Add(20 * time.Second)},
		{EventID: "r2", IsReport: true, PrescriptionID: "p2", ExitCode: intPtr(0), Timestamp: now.Add(30 * time.Second)},
	}

	got := DetectRepairLoop(entries)
	if got.Count != 1 {
		t.Fatalf("repair_loop count=%d, want 1", got.Count)
	}
	assertEventID(t, got.EventIDs, "p2")
}

func TestDetectRepairLoop_NoArtifactChange(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []Entry{
		{EventID: "p1", IsPrescription: true, ActorID: "a", IntentDigest: "i1", ArtifactDigest: "sha256:a", Timestamp: now},
		{EventID: "r1", IsReport: true, PrescriptionID: "p1", ExitCode: intPtr(1), Timestamp: now.Add(10 * time.Second)},
		{EventID: "p2", IsPrescription: true, ActorID: "a", IntentDigest: "i1", ArtifactDigest: "sha256:a", Timestamp: now.Add(20 * time.Second)},
		{EventID: "r2", IsReport: true, PrescriptionID: "p2", ExitCode: intPtr(0), Timestamp: now.Add(30 * time.Second)},
	}

	got := DetectRepairLoop(entries)
	if got.Count != 0 {
		t.Fatalf("repair_loop count=%d, want 0", got.Count)
	}
}

func TestDetectThrashing(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []Entry{
		{EventID: "p1", IsPrescription: true, ActorID: "a", IntentDigest: "i1", Timestamp: now},
		{EventID: "r1", IsReport: true, PrescriptionID: "p1", ExitCode: intPtr(1), Timestamp: now.Add(5 * time.Second)},
		{EventID: "p2", IsPrescription: true, ActorID: "a", IntentDigest: "i2", Timestamp: now.Add(10 * time.Second)},
		{EventID: "r2", IsReport: true, PrescriptionID: "p2", ExitCode: intPtr(1), Timestamp: now.Add(15 * time.Second)},
		{EventID: "p3", IsPrescription: true, ActorID: "a", IntentDigest: "i3", Timestamp: now.Add(20 * time.Second)},
		{EventID: "r3", IsReport: true, PrescriptionID: "p3", ExitCode: intPtr(1), Timestamp: now.Add(25 * time.Second)},
	}

	got := DetectThrashing(entries)
	if got.Count != 3 {
		t.Fatalf("thrashing count=%d, want 3", got.Count)
	}
}

func TestDetectThrashing_ResetOnSuccess(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []Entry{
		{EventID: "p1", IsPrescription: true, ActorID: "a", IntentDigest: "i1", Timestamp: now},
		{EventID: "r1", IsReport: true, PrescriptionID: "p1", ExitCode: intPtr(1), Timestamp: now.Add(5 * time.Second)},
		{EventID: "p2", IsPrescription: true, ActorID: "a", IntentDigest: "i2", Timestamp: now.Add(10 * time.Second)},
		{EventID: "r2", IsReport: true, PrescriptionID: "p2", ExitCode: intPtr(0), Timestamp: now.Add(15 * time.Second)},
		{EventID: "p3", IsPrescription: true, ActorID: "a", IntentDigest: "i3", Timestamp: now.Add(20 * time.Second)},
		{EventID: "r3", IsReport: true, PrescriptionID: "p3", ExitCode: intPtr(1), Timestamp: now.Add(25 * time.Second)},
		{EventID: "p4", IsPrescription: true, ActorID: "a", IntentDigest: "i4", Timestamp: now.Add(30 * time.Second)},
		{EventID: "r4", IsReport: true, PrescriptionID: "p4", ExitCode: intPtr(1), Timestamp: now.Add(35 * time.Second)},
		{EventID: "p5", IsPrescription: true, ActorID: "a", IntentDigest: "i5", Timestamp: now.Add(40 * time.Second)},
		{EventID: "r5", IsReport: true, PrescriptionID: "p5", ExitCode: intPtr(1), Timestamp: now.Add(45 * time.Second)},
	}

	got := DetectThrashing(entries)
	if got.Count != 3 {
		t.Fatalf("thrashing count=%d, want 3 after reset", got.Count)
	}
}
