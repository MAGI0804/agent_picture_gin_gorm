//go:build googlee2e
// +build googlee2e

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "gin-biz-web-api/config"
	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/service/agent_svc"
	"gin-biz-web-api/model"
	pkgconfig "gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/database"

	"gorm.io/driver/mysql"
	"gorm.io/gorm/logger"
)

func TestGoogleModelEndToEnd(t *testing.T) {
	setupGoogleE2EDatabase(t)

	userID, textModelConfigID, imageModelConfigID := googleE2ETarget(t)
	t.Logf(
		"google e2e target user_id=%d text_model_config_id=%d image_model_config_id=%d",
		userID,
		textModelConfigID,
		imageModelConfigID,
	)
	conversation, err := agent_svc.NewAgentService().CreateConversation(
		userID,
		fmt.Sprintf("Google real provider E2E %s", time.Now().Format("20060102150405")),
	)
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := NewService()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	result, err := svc.CreateRun(ctx, userID, conversation.ID, CreateRunRequest{
		Content:            "生成一张简洁的产品海报：白色陶瓷咖啡杯放在浅灰桌面上，柔和自然光，干净高级，画面不要出现文字。",
		TaskType:           "image_generation",
		IdempotencyKey:     fmt.Sprintf("google-e2e-%d", time.Now().UnixNano()),
		TextModelConfigID:  textModelConfigID,
		ImageModelConfigID: imageModelConfigID,
		CandidateCount:     1,
	})
	if err != nil {
		t.Fatalf(
			"create run with real google provider using text_model_config_id=%d image_model_config_id=%d: %v",
			textModelConfigID,
			imageModelConfigID,
			err,
		)
	}

	run, ok := result["agent_run"].(model.AgentRun)
	if !ok || run.ID == 0 {
		t.Fatalf("agent_run missing from result: %#v", result["agent_run"])
	}
	artifacts, ok := result["artifacts"].([]model.Artifact)
	if !ok || len(artifacts) == 0 {
		t.Fatalf("artifacts missing from result: %#v", result["artifacts"])
	}
	artifact := artifacts[0]
	if artifact.ID == 0 || artifact.PreviewURL == "" {
		t.Fatalf("artifact did not expose an owned preview URL: %#v", artifact)
	}

	versions, err := svc.ListArtifactVersions(userID, artifact.ID)
	if err != nil {
		t.Fatalf("list artifact versions: %v", err)
	}
	if len(versions) == 0 {
		t.Fatalf("artifact %d has no versions", artifact.ID)
	}
	version := versions[0]
	if strings.TrimSpace(version.QualityScores) == "" {
		t.Fatalf("artifact version %d has no review quality_scores", version.ID)
	}

	if err := svc.SelectArtifact(userID, artifact.ID, SelectArtifactRequest{ArtifactVersionID: version.ID}); err != nil {
		t.Fatalf("select artifact: %v", err)
	}
	if err := svc.RecordArtifactFeedback(userID, artifact.ID, ArtifactFeedbackRequest{
		ArtifactVersionID: version.ID,
		FeedbackType:      "positive",
		Rating:            5,
		Comment:           "Google real provider E2E verification.",
	}); err != nil {
		t.Fatalf("record artifact feedback: %v", err)
	}

	_, downloadPath, err := svc.DownloadArtifact(userID, artifact.ID)
	if err != nil {
		t.Fatalf("authorize download: %v", err)
	}
	info, err := os.Stat(downloadPath)
	if err != nil {
		t.Fatalf("stat downloaded artifact path %s: %v", downloadPath, err)
	}
	if info.Size() == 0 {
		t.Fatalf("downloaded artifact path %s is empty", downloadPath)
	}
	_, previewPath, err := svc.PreviewArtifact(userID, artifact.ID)
	if err != nil {
		t.Fatalf("authorize preview: %v", err)
	}
	if previewPath != downloadPath {
		t.Fatalf("preview path = %q, want download path %q", previewPath, downloadPath)
	}

	t.Logf(
		"google e2e ok user_id=%d conversation_id=%d run_id=%d artifact_id=%d version_id=%d provider=%s image_model=%s bytes=%d preview_url=%s",
		userID,
		conversation.ID,
		run.ID,
		artifact.ID,
		version.ID,
		version.ModelProvider,
		version.ModelName,
		info.Size(),
		artifact.PreviewURL,
	)
}

func setupGoogleE2EDatabase(t *testing.T) {
	t.Helper()

	root := findGoogleE2EProjectRoot(t)
	global.RootPath = root
	pkgconfig.NewConfig("", filepath.Join(root, "etc")+string(os.PathSeparator))

	dbConfig := googleE2EDBConfig()
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local&collation=%s",
		dbConfig.username,
		dbConfig.password,
		dbConfig.host,
		dbConfig.port,
		dbConfig.database,
		dbConfig.charset,
		dbConfig.collation,
	)
	database.ConnectMySQL(map[string]*database.DBClientConfig{
		"default": {
			DBConfig:        mysql.New(mysql.Config{DSN: dsn}),
			LG:              logger.Default.LogMode(logger.Silent),
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 5 * time.Minute,
		},
	})
	t.Cleanup(database.Close)
}

type googleE2EDatabaseConfig struct {
	host      string
	port      string
	database  string
	username  string
	password  string
	charset   string
	collation string
}

func googleE2EDBConfig() googleE2EDatabaseConfig {
	return googleE2EDatabaseConfig{
		host:      pkgconfig.GetString("cfg.database.mysql.default.host"),
		port:      pkgconfig.GetString("cfg.database.mysql.default.port"),
		database:  pkgconfig.GetString("cfg.database.mysql.default.database"),
		username:  pkgconfig.GetString("cfg.database.mysql.default.username"),
		password:  pkgconfig.GetString("cfg.database.mysql.default.password"),
		charset:   pkgconfig.GetString("cfg.database.mysql.default.charset", "utf8mb4"),
		collation: pkgconfig.GetString("cfg.database.mysql.default.collation", "utf8mb4_0900_ai_ci"),
	}
}

func findGoogleE2EProjectRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "etc", "config.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find project root containing etc/config.yaml from %s", dir)
		}
		dir = parent
	}
}

func googleE2ETarget(t *testing.T) (uint, uint, uint) {
	t.Helper()

	userID := uintFromEnv("GOOGLE_E2E_USER_ID")
	textModelConfigID := uintFromEnv("GOOGLE_E2E_TEXT_MODEL_CONFIG_ID")
	imageModelConfigID := uintFromEnv("GOOGLE_E2E_IMAGE_MODEL_CONFIG_ID")

	if userID == 0 {
		var selected model.UserModelConfig
		err := database.DB.
			Where("selected_text_model_config_id > 0 AND selected_image_model_config_id > 0").
			Order("updated_at desc, id desc").
			First(&selected).Error
		if err == nil {
			userID = selected.UserID
			if textModelConfigID == 0 {
				textModelConfigID = selected.SelectedTextModelConfigID
			}
			if imageModelConfigID == 0 {
				imageModelConfigID = selected.SelectedImageModelConfigID
			}
		}
	}
	if userID == 0 {
		var user model.User
		if err := database.DB.Order("id asc").First(&user).Error; err != nil {
			t.Fatalf("find a user for E2E: %v", err)
		}
		userID = user.ID
	}
	if textModelConfigID == 0 {
		textModelConfigID = findGoogleModelConfig(t, true, false, "gemini-3.5-flash", "text")
	}
	if imageModelConfigID == 0 {
		imageModelConfigID = findGoogleModelConfig(t, false, true, "imagen-4.0-ultra-generate-001", "image_generation")
	}

	assertGoogleModelConfig(t, textModelConfigID, "text")
	assertGoogleModelConfig(t, imageModelConfigID, "image_generation")
	return userID, textModelConfigID, imageModelConfigID
}

func findGoogleModelConfig(t *testing.T, isText bool, isImage bool, modelName string, capability string) uint {
	t.Helper()

	var configs []model.ModelConfig
	if err := database.DB.
		Where("is_text_model = ? AND is_image_model = ? AND model_name = ?", isText, isImage, modelName).
		Order("id desc").
		Find(&configs).Error; err != nil {
		t.Fatalf("find model configs: %v", err)
	}
	for _, config := range configs {
		if googleConfigString(config.ConfigInfo, "provider") == "google" &&
			googleConfigString(config.ConfigInfo, "capability") == capability &&
			googleConfigString(config.ConfigInfo, "api_key") != "" {
			return config.ID
		}
	}
	t.Fatalf("no google %s model config found for %s with api_key", capability, modelName)
	return 0
}

func assertGoogleModelConfig(t *testing.T, id uint, capability string) {
	t.Helper()

	var config model.ModelConfig
	if err := database.DB.Where("id = ?", id).First(&config).Error; err != nil {
		t.Fatalf("load model config %d: %v", id, err)
	}
	if googleConfigString(config.ConfigInfo, "provider") != "google" {
		t.Fatalf("model config %d provider is not google", id)
	}
	apiKey := googleConfigString(config.ConfigInfo, "api_key")
	t.Logf(
		"google e2e config id=%d capability=%s model=%s request_url=%s api_type=%s base_url=%s api_key_length=%d api_key_sha256=%s",
		id,
		capability,
		config.ModelName,
		config.RequestURL,
		googleConfigString(config.ConfigInfo, "api_type"),
		googleConfigString(config.ConfigInfo, "base_url"),
		len(apiKey),
		shortSHA256(apiKey),
	)
	if apiKey == "" {
		t.Fatalf("model config %d has no api_key", id)
	}
	if strings.Contains(apiKey, "<") || strings.Contains(strings.ToLower(apiKey), "your") || strings.Contains(apiKey, "正式") {
		t.Fatalf("model config %d api_key still looks like a placeholder", id)
	}
	if got := googleConfigString(config.ConfigInfo, "capability"); got != "" && got != capability {
		t.Fatalf("model config %d capability = %q, want %q", id, got, capability)
	}
}

func googleConfigString(config model.JSONMap, key string) string {
	value, ok := config[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func shortSHA256(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

func uintFromEnv(name string) uint {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return uint(parsed)
}
