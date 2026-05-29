package agents

import (
	"bytes"
	"image"
	"reflect"
	"strings"
	"testing"

	artifactsvc "gin-biz-web-api/internal/service/agent_v2/artifact"
	"gin-biz-web-api/internal/service/agent_v2/domain"
	"gin-biz-web-api/model"
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

func TestImageCompositionAgentUsesInputArtifactIDsInRequestOrder(t *testing.T) {
	store := &fakeCompositionStore{
		artifacts: []model.Artifact{
			{BaseModel: model.BaseModel{ID: 1}, Kind: "image", ObjectKey: "old-template.png"},
			{BaseModel: model.BaseModel{ID: 2}, Kind: "image", ObjectKey: "icon.png"},
			{BaseModel: model.BaseModel{ID: 3}, Kind: "image", ObjectKey: "template.png"},
		},
	}
	agent := NewImageCompositionAgent(store)

	got, err := agent.uploadedImages(domain.RunState{
		UserID:         7,
		ConversationID: 8,
		Metadata: map[string]string{
			"input_artifact_ids": "[3,2]",
		},
	})
	if err != nil {
		t.Fatalf("uploadedImages() error = %v", err)
	}
	if len(got) != 2 || got[0].ID != 3 || got[1].ID != 2 {
		t.Fatalf("uploadedImages() = %#v, want artifacts 3 then 2", got)
	}
}

func TestImageCompositionAgentSelectsAIEditBaseBeforeFinalComposition(t *testing.T) {
	store := &fakeCompositionStore{
		artifacts: []model.Artifact{
			{BaseModel: model.BaseModel{ID: 1}, Kind: "image", ObjectKey: "template.jpg"},
			{BaseModel: model.BaseModel{ID: 2}, Kind: "image", ObjectKey: "logo.png"},
			{
				BaseModel:        model.BaseModel{ID: 3},
				AgentRunID:       42,
				Kind:             "image",
				ObjectKey:        "ai-edit.png",
				ParentArtifactID: 1,
				ArtifactGroupID:  "run-42-ai-edits",
				RankScore:        1,
			},
		},
	}
	agent := NewImageCompositionAgent(store)

	got, err := agent.compositionInputs(domain.RunState{
		RunID:          42,
		UserID:         7,
		ConversationID: 8,
		Metadata: map[string]string{
			"input_artifact_ids": "[1,2]",
		},
	})
	if err != nil {
		t.Fatalf("compositionInputs() error = %v", err)
	}
	if got.Template.ID != 1 || got.Base.ID != 3 || len(got.Icons) != 1 || got.Icons[0].ID != 2 {
		t.Fatalf("compositionInputs() = %#v, want template 1, AI base 3, icon 2", got)
	}
}

func TestParseExactTemplateLayoutKeepsOnlyRequestedText(t *testing.T) {
	request := `基于现有儿童服装产品图片模板，精准添加指定文字元素 + LOGO 元素，严格按照位置、像素边距、字体、字号、色值、排列方式、字重规范制作。不要把色值打上去了

1. 左上角文字设置
● 定位：左上角，距左边缘 50px、距上边缘 50px
● 文字内容：Do Small Things
● 字体：Arial
● 字号：16 号
● 字体颜色：灰色 #b6b6b4
2. 右上角文字设置
● 定位：右上角，距右边缘 50px、距上边缘 50px
● 文字内容：With Great Love
● 字体：Arial
● 字号：16 号
● 字体颜色：灰色 #b6b6b4
3. 左侧中部竖向文字设置
● 定位：左侧中部，距左边缘 80px、距上边缘 250px
① 文字：桉树麻字体：无衬线体字号：28 号字体颜色：蓝色 #00b5e2字重：加粗大字 竖向排列
② 文字：POLO 背心字体：无衬线体字号：26 号字体颜色：黑色 #242425字重：常规大字 竖向排列
4. 右侧中部竖向文字设置
● 定位：右侧中部，距右边缘 80px、距上边缘 320px
① 右侧文字：夏季的黄金配比 \  字体：无衬线体字号：20号主体颜色：黑色 #4e4e4f特殊符号：斜杠 \ 单独上色 #03B3E2字重：常规中字  竖向排列
② 左侧文字：\ 桉树麻 = 吸湿 + 透汽 + 柔软字体：无衬线体字号：20 号主体颜色：黑色 #4e4e4f特殊符号：斜杠 \单独上色 #03B3E2字重：常规中字  竖向排列
5. 左下角 LOGO 设置
● 定位：左下角，距左边缘 50px、距下边缘 100px
● 元素：放置指定 LOGO 图片，比例适配不变形、不遮挡主体画面`

	layout := parseExactTemplateLayout(request)
	if !layout.IsExact {
		t.Fatal("parseExactTemplateLayout().IsExact = false, want true")
	}
	got := layout.Texts()
	want := []string{
		"Do Small Things",
		"With Great Love",
		"桉树麻",
		"POLO 背心",
		"夏季的黄金配比 \\",
		"\\ 桉树麻 = 吸湿 + 透汽 + 柔软",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("layout.Texts() = %#v, want %#v", got, want)
	}
	for _, text := range got {
		if strings.Contains(text, "字体") || strings.Contains(text, "字号") || strings.Contains(text, "#") {
			t.Fatalf("layout text includes instruction metadata: %q", text)
		}
	}
	if layout.TopLeft.X != 50 || layout.TopLeft.Y != 50 || layout.TopLeft.Size != 16 {
		t.Fatalf("top-left layout = %#v, want 50/50/16", layout.TopLeft)
	}
	if layout.Logo.X != 50 || layout.Logo.Bottom != 100 {
		t.Fatalf("logo layout = %#v, want left 50 bottom 100", layout.Logo)
	}
}

func TestScaleExactTemplateLayoutForLargeTemplateMakesTextReadable(t *testing.T) {
	layout := exactTemplateLayout{
		IsExact: true,
		TopLeft: horizontalTextSpec{X: 50, Y: 50, Size: 16},
		TopRight: horizontalTextSpec{
			RightEdge: 50,
			Y:         50,
			Size:      16,
		},
		LeftColumns: []verticalTextSpec{{X: 80, Y: 250, Size: 28}},
		RightRight:  verticalTextSpec{X: 80, Y: 320, Size: 20},
		RightLeft:   verticalTextSpec{X: 80, Y: 320, Size: 20},
		Logo:        logoSpec{X: 50, Bottom: 100},
	}

	got := scaleExactTemplateLayoutForCanvas(layout, image.Rect(0, 0, 4500, 6000))

	if got.TopLeft.Size != 60 || got.LeftColumns[0].Size != 105 || got.RightRight.Size != 75 {
		t.Fatalf("scaled sizes = top %.1f left %.1f right %.1f, want 60/105/75", got.TopLeft.Size, got.LeftColumns[0].Size, got.RightRight.Size)
	}
	if got.TopLeft.X != 188 || got.TopLeft.Y != 188 || got.Logo.Bottom != 375 {
		t.Fatalf("scaled positions = top-left %d,%d logo bottom %d, want 188,188,375", got.TopLeft.X, got.TopLeft.Y, got.Logo.Bottom)
	}
	if got.Logo.Scale != 3.75 {
		t.Fatalf("logo scale = %.2f, want 3.75", got.Logo.Scale)
	}
}

func TestEncodeComposedImagePreservesTemplateJPEGFormatAndSize(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, 123, 234))
	content, mimeType, fileName, err := encodeComposedImage(canvas, model.Artifact{
		Name:     "template.jpg",
		MimeType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("encodeComposedImage() error = %v", err)
	}
	if mimeType != "image/jpeg" || fileName != "composed-image.jpg" {
		t.Fatalf("encoded output = %q %q, want jpeg composed-image.jpg", mimeType, fileName)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("DecodeConfig() error = %v", err)
	}
	if config.Width != 123 || config.Height != 234 {
		t.Fatalf("encoded size = %dx%d, want 123x234", config.Width, config.Height)
	}
}

type fakeCompositionStore struct {
	artifacts []model.Artifact
}

func (store *fakeCompositionStore) ListArtifacts(userID uint, conversationID uint) ([]model.Artifact, error) {
	return store.artifacts, nil
}

func (store *fakeCompositionStore) CreateRenderedArtifact(input artifactsvc.CreateRenderedArtifactInput) (model.Artifact, model.ArtifactVersion, error) {
	return model.Artifact{}, model.ArtifactVersion{}, nil
}
