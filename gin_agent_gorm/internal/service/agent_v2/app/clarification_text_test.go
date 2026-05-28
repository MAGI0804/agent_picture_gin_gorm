package app

import (
	"strings"
	"testing"
)

func TestAppendClarificationToRequestUsesChineseLabels(t *testing.T) {
	result := appendClarificationToRequest(
		"生成一张风景照",
		[]string{"图片的主体应该是什么？", "希望采用什么风格、用途或画面比例？"},
		"海滩，16:9",
	)

	for _, want := range []string{"补充信息：", "- 图片的主体应该是什么？", "回答：海滩，16:9"} {
		if !strings.Contains(result, want) {
			t.Fatalf("clarification text = %q, want %q", result, want)
		}
	}
	if strings.Contains(result, "Clarification:") || strings.Contains(result, "Answer:") {
		t.Fatalf("clarification text contains English labels: %q", result)
	}
}
