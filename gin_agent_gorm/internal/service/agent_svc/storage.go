package agent_svc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gin-biz-web-api/global"
	"gin-biz-web-api/pkg/config"
)

// StoredObject 表示对象存储保存后的文件信息。
type StoredObject struct {
	ObjectKey  string
	PreviewURL string
	SizeBytes  int64
	Hash       string
}

// ObjectStore 定义产物文件存储接口，后续可替换为 S3 兼容对象存储。
type ObjectStore interface {
	Save(objectKey string, content []byte) (StoredObject, error)
	Path(objectKey string) string
}

// LocalObjectStore 使用本地磁盘保存生成产物。
type LocalObjectStore struct {
	rootPath  string
	publicURL string
}

// NewObjectStore 根据配置创建对象存储实例。
func NewObjectStore() ObjectStore {
	rootPath := config.GetString("cfg.ai_agent.storage.local_path", "public/artifacts")
	if !filepath.IsAbs(rootPath) {
		rootPath = filepath.Join(global.RootPath, rootPath)
	}
	return &LocalObjectStore{
		rootPath:  rootPath,
		publicURL: config.GetString("cfg.ai_agent.storage.public_path", "/artifacts"),
	}
}

// Save 将文件内容写入本地磁盘，并返回预览地址、大小和 hash。
func (store *LocalObjectStore) Save(objectKey string, content []byte) (StoredObject, error) {
	normalizedKey, err := normalizeObjectKey(objectKey)
	if err != nil {
		return StoredObject{}, err
	}
	objectKey = normalizedKey
	fullPath := store.Path(objectKey)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return StoredObject{}, err
	}
	if err := ioutil.WriteFile(fullPath, content, 0644); err != nil {
		return StoredObject{}, err
	}
	hash := sha256.Sum256(content)
	return StoredObject{
		ObjectKey:  objectKey,
		PreviewURL: store.publicURL + "/" + filepath.ToSlash(objectKey),
		SizeBytes:  int64(len(content)),
		Hash:       hex.EncodeToString(hash[:]),
	}, nil
}

// Path 根据对象 key 返回本地文件绝对路径。
func (store *LocalObjectStore) Path(objectKey string) string {
	cleanKey, err := normalizeObjectKey(objectKey)
	if err != nil {
		cleanKey = "invalid-object-key"
	}
	return filepath.Join(store.rootPath, cleanKey)
}

func normalizeObjectKey(objectKey string) (string, error) {
	objectKey = filepath.ToSlash(strings.TrimSpace(objectKey))
	objectKey = strings.TrimPrefix(objectKey, "/")
	for _, part := range strings.Split(objectKey, "/") {
		if part == ".." {
			return "", errors.New("object key contains unsafe path segment")
		}
	}
	cleanKey := filepath.ToSlash(filepath.Clean(objectKey))
	if cleanKey == "." || cleanKey == "" {
		return "", errors.New("object key is empty")
	}
	for _, part := range strings.Split(cleanKey, "/") {
		if part == ".." || part == "" {
			return "", errors.New("object key contains unsafe path segment")
		}
	}
	if filepath.IsAbs(cleanKey) || strings.Contains(cleanKey, ":") {
		return "", errors.New("object key must be relative")
	}
	return filepath.FromSlash(cleanKey), nil
}
