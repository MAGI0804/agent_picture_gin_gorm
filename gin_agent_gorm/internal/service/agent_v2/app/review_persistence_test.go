package app

import (
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
)

func TestCandidateReviewsForPersistenceUsesPerCandidateRankedReviews(t *testing.T) {
	state := domain.RunState{
		Artifacts: []domain.ArtifactRef{
			{ID: 1, VersionID: 10, PreviewURL: "/preview/1"},
			{ID: 2, VersionID: 20, PreviewURL: "/preview/2"},
		},
		Review: domain.ReviewResult{
			OverallScore: 0.30,
			CandidateReviews: []domain.CandidateReview{
				{ArtifactID: 2, VersionID: 20, OverallScore: 0.91, RankScore: 0.97},
				{ArtifactID: 1, VersionID: 10, OverallScore: 0.77, RankScore: 0.80},
			},
		},
	}

	reviews := candidateReviewsForPersistence(state)
	if len(reviews) != 2 {
		t.Fatalf("reviews = %#v, want 2", reviews)
	}
	if reviews[0].ArtifactID != 2 || reviews[0].RankScore != 0.97 {
		t.Fatalf("first review = %#v, want ranked candidate review for artifact 2", reviews[0])
	}
	if reviews[1].OverallScore != 0.77 {
		t.Fatalf("second score = %f, want candidate-specific score 0.77", reviews[1].OverallScore)
	}
}

func TestCandidateReviewsForPersistenceFallsBackToOverallReview(t *testing.T) {
	state := domain.RunState{
		Artifacts: []domain.ArtifactRef{
			{ID: 1, VersionID: 10, PreviewURL: "/preview/1"},
			{ID: 2, VersionID: 20, PreviewURL: "/preview/2"},
		},
		Review: domain.ReviewResult{
			OverallScore: 0.82,
			Issues:       []string{"minor crop"},
			Reviewer:     "mock_vision_review",
		},
	}

	reviews := candidateReviewsForPersistence(state)
	if len(reviews) != 2 {
		t.Fatalf("reviews = %#v, want fallback review per artifact", reviews)
	}
	if reviews[1].ArtifactID != 2 || reviews[1].OverallScore != 0.82 {
		t.Fatalf("fallback review = %#v, want artifact 2 with overall score", reviews[1])
	}
}
