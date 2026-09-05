package llm

import (
	"encoding/json"
	"testing"
)

func TestMockScoreRejectsNonsense(t *testing.T) {
	raw := mockScoreJSON("Role: Backend Engineer\nQuestion: What is a goroutine?\nCandidate answer: a b c d")
	var got struct {
		Clarity     int `json:"clarity"`
		Correctness int `json:"correctness"`
		Depth       int `json:"depth"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.Clarity != 0 || got.Correctness != 0 || got.Depth != 0 {
		t.Fatalf("expected zero scores for nonsense, got %+v", got)
	}
}

func TestMockScoreExtractsCandidateAnswerOnly(t *testing.T) {
	raw := mockScoreJSON("Role: Backend Engineer\nQuestion: Explain goroutines and channels in Go.\nCandidate answer: goroutines run concurrently and channels coordinate communication between goroutines")
	var got struct {
		Correctness int `json:"correctness"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.Correctness < 4 {
		t.Fatalf("expected a relevant answer to score above nonsense, got %d", got.Correctness)
	}
}

func TestMockResumeUsesUploadedText(t *testing.T) {
	raw := mockResumeJSON("Resume text:\nSagun Patwari\nSoftware Engineer\nGo PostgreSQL Docker Kafka")
	var got struct {
		Name   string   `json:"name"`
		Skills []string `json:"skills"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Sagun Patwari" {
		t.Fatalf("expected uploaded name, got %q", got.Name)
	}
	if len(got.Skills) == 0 {
		t.Fatal("expected detected skills")
	}
}
