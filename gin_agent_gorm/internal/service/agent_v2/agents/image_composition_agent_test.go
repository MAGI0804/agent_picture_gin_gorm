package agents

import (
	"reflect"
	"testing"

	"gin-biz-web-api/internal/service/agent_v2/domain"
)

func TestExtractQuotedTexts(t *testing.T) {
	got := extractQuotedTexts("template text \u201cNew Arrival\u201d and \"Limited Offer\"")
	want := []string{"New Arrival", "Limited Offer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractQuotedTexts() = %#v, want %#v", got, want)
	}
}

func TestCompositionTextLinesFallsBackToRequest(t *testing.T) {
	request := "compose final image from uploaded assets"
	got := compositionTextLines(domain.RunState{
		UserRequest: request,
	})
	want := []string{truncateRunes(request, 28)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("compositionTextLines() = %#v, want %#v", got, want)
	}
}
