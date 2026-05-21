package model_request

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	JimengReqKey  = "jimeng_seedream46_cvtob"
	JimengAction  = "CVSync2AsyncSubmitTask"
	JimengVersion = "2022-08-31"
)

// JimengConfig 即梦AI认证配置。
type JimengConfig struct {
	AccessKeyID     string // 访问密钥ID
	SecretAccessKey string // 访问密钥Secret
	Region          string // 区域，例如 "cn-north-1"
	Service         string // 服务名称，例如 "jimeng"
}

// JimengImageRequest 即梦AI图片生成请求参数。
type JimengImageRequest struct {
	ReqKey     string   `json:"req_key"`      // 服务标识，固定值: jimeng_seedream46_cvtob
	ImageURLs  []string `json:"image_urls"`   // 图片文件URL，支持0-14张图
	Prompt     string   `json:"prompt"`       // 用于生成图像的提示词，中英文均可，最长不超过800字符
	Size       int      `json:"size,omitempty"`      // 生成图片的面积，默认4194304(2048*2048)，范围[1024*1024, 4096*4096]
	Width      int      `json:"width,omitempty"`     // 生成图像宽度，需与height同时传才生效
	Height     int      `json:"height,omitempty"`    // 生成图像高度，需与width同时传才生效
	Scale      int      `json:"scale,omitempty"`     // 文本描述影响程度，默认50，范围[1,100]
	ForceSingle bool    `json:"force_single,omitempty"` // 是否强制生成单图，默认false
	MinRatio   float64  `json:"min_ratio,omitempty"`   // 生图宽高比最小值，默认1/3，范围[1/16,16)
	MaxRatio   float64  `json:"max_ratio,omitempty"`   // 生图宽高比最大值，默认3，范围(1/16,16]
	CallbackURL string  `json:"callback_url,omitempty"` // 回调接口URL（异步回调时使用）
	ReturnURL  bool    `json:"return_url,omitempty"`   // 是否以链接形式返回图片，默认false（异步回调时使用）
	LogoInfo   string  `json:"logo_info,omitempty"`    // 水印信息，JSON字符串（异步回调时使用）
	AIGCMeta   string  `json:"aigc_meta,omitempty"`    // 隐式标识，JSON字符串（异步回调时使用）
}

// JimengImageResponse 即梦AI图片生成响应结果。
type JimengImageResponse struct {
	Code       int    `json:"code"`        // 状态码，10000表示成功
	Data       struct {
		TaskID string `json:"task_id"`   // 任务ID，用于查询接口
	} `json:"data"`
	Message    string `json:"message"`     // 响应消息
	RequestID  string `json:"request_id"`  // 请求ID，排查错误使用
	TimeElapsed string `json:"time_elapsed"` // 链路耗时
}

// hmacSHA256 计算HMAC-SHA256哈希值。
func hmacSHA256(key []byte, content string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))
	return mac.Sum(nil)
}

// getSignedKey 生成签名密钥。
func getSignedKey(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte(secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "request")
	return kSigning
}

// hashSHA256 计算SHA256哈希值。
func hashSHA256(data []byte) []byte {
	hash := sha256.New()
	hash.Write(data)
	return hash.Sum(nil)
}

// signJimengRequest 为即梦AI请求添加签名Header。
func signJimengRequest(request *http.Request, config JimengConfig, body []byte) error {
	now := time.Now()
	date := now.UTC().Format("20060102T150405Z")
	authDate := date[:8]

	// 设置X-Date头
	request.Header.Set("X-Date", date)

	// 计算请求体的哈希并设置X-Content-Sha256头
	payload := hex.EncodeToString(hashSHA256(body))
	request.Header.Set("X-Content-Sha256", payload)

	// 设置Content-Type
	request.Header.Set("Content-Type", "application/json")

	// 获取路径和查询参数
	u := request.URL
	path := u.Path
	if path == "" { // 如果路径为空，使用根路径
		path = "/"
	}

	// 构建查询字符串
	queryString := strings.Replace(u.RawQuery, "+", "%20", -1)

	// 需要签名的Header
	signedHeaders := []string{"host", "x-date", "x-content-sha256", "content-type"}

	var headerList []string
	for _, header := range signedHeaders {
		if header == "host" {
			headerList = append(headerList, header+":"+request.Host)
		} else {
			v := request.Header.Get(header)
			headerList = append(headerList, header+":"+strings.TrimSpace(v))
		}
	}
	headerString := strings.Join(headerList, "\n")

	// 构建规范请求字符串
	canonicalString := strings.Join([]string{
		request.Method,
		path,
		queryString,
		headerString + "\n",
		strings.Join(signedHeaders, ";"),
		payload,
	}, "\n")

	// 构建待签名字符串
	hashedCanonicalString := hex.EncodeToString(hashSHA256([]byte(canonicalString)))
	credentialScope := authDate + "/" + config.Region + "/" + config.Service + "/request"
	signString := strings.Join([]string{
		"HMAC-SHA256",
		date,
		credentialScope,
		hashedCanonicalString,
	}, "\n")

	// 计算签名
	signedKey := getSignedKey(config.SecretAccessKey, authDate, config.Region, config.Service)
	signature := hex.EncodeToString(hmacSHA256(signedKey, signString))

	// 构建Authorization头
	authorization := "HMAC-SHA256" + " Credential=" + config.AccessKeyID + "/" + credentialScope +
		", SignedHeaders=" + strings.Join(signedHeaders, ";") +
		", Signature=" + signature
	request.Header.Set("Authorization", authorization)

	return nil
}

// SendJimengImageRequest 调用即梦AI-图片生成4.6模型。
// 
// 参数:
//   - baseURL: 即梦AI API的基础地址
//   - config: 认证配置
//   - request: 图片生成请求参数
//
// 返回:
//   - JimengImageResponse: 响应结果，包含task_id用于后续查询
//   - error: 错误信息
func SendJimengImageRequest(baseURL string, config JimengConfig, request JimengImageRequest) (JimengImageResponse, error) {
	if strings.TrimSpace(baseURL) == "" {
		return JimengImageResponse{}, errors.New("baseURL cannot be empty")
	}

	if strings.TrimSpace(config.AccessKeyID) == "" {
		return JimengImageResponse{}, errors.New("AccessKeyID cannot be empty")
	}

	if strings.TrimSpace(config.SecretAccessKey) == "" {
		return JimengImageResponse{}, errors.New("SecretAccessKey cannot be empty")
	}

	if strings.TrimSpace(config.Region) == "" {
		return JimengImageResponse{}, errors.New("Region cannot be empty")
	}

	if strings.TrimSpace(config.Service) == "" {
		return JimengImageResponse{}, errors.New("Service cannot be empty")
	}

	if request.ReqKey == "" {
		request.ReqKey = JimengReqKey
	}

	if request.ReqKey != JimengReqKey {
		return JimengImageResponse{}, errors.New("invalid req_key, must be: " + JimengReqKey)
	}

	if strings.TrimSpace(request.Prompt) == "" {
		return JimengImageResponse{}, errors.New("prompt cannot be empty")
	}

	if len(request.Prompt) > 800 {
		return JimengImageResponse{}, errors.New("prompt cannot exceed 800 characters")
	}

	if request.Size < 0 {
		return JimengImageResponse{}, errors.New("size must be non-negative")
	}
	if request.Size > 0 && (request.Size < 1024*1024 || request.Size > 4096*4096) {
		return JimengImageResponse{}, errors.New("size must be between 1024*1024 and 4096*4096")
	}

	if request.Width > 0 && request.Height > 0 {
		area := request.Width * request.Height
		if area < 1024*1024 || area > 4096*4096 {
			return JimengImageResponse{}, errors.New("width * height must be between 1024*1024 and 4096*4096")
		}
		ratio := float64(request.Width) / float64(request.Height)
		minRatio := request.MinRatio
		if minRatio == 0 {
			minRatio = 1.0 / 3.0
		}
		maxRatio := request.MaxRatio
		if maxRatio == 0 {
			maxRatio = 3.0
		}
		if ratio < minRatio || ratio > maxRatio {
			return JimengImageResponse{}, errors.New("width/height ratio must be between min_ratio and max_ratio")
		}
	}

	if request.Scale < 0 || request.Scale > 100 {
		return JimengImageResponse{}, errors.New("scale must be between 1 and 100")
	}
	if request.Scale == 0 {
		request.Scale = 50
	}

	if request.MinRatio == 0 {
		request.MinRatio = 1.0 / 3.0
	}
	if request.MaxRatio == 0 {
		request.MaxRatio = 3.0
	}

	if len(request.ImageURLs) > 14 {
		return JimengImageResponse{}, errors.New("image_urls can contain at most 14 images")
	}

	apiURL, err := buildJimengURL(baseURL)
	if err != nil {
		return JimengImageResponse{}, errors.Wrap(err, "failed to build URL")
	}

	payload := map[string]interface{}{
		"req_key": request.ReqKey,
		"prompt":  request.Prompt,
	}

	if len(request.ImageURLs) > 0 {
		payload["image_urls"] = request.ImageURLs
	}
	if request.Size > 0 {
		payload["size"] = request.Size
	}
	if request.Width > 0 && request.Height > 0 {
		payload["width"] = request.Width
		payload["height"] = request.Height
	}
	if request.Scale > 0 {
		payload["scale"] = request.Scale
	}
	if request.ForceSingle {
		payload["force_single"] = request.ForceSingle
	}
	if request.MinRatio > 0 {
		payload["min_ratio"] = request.MinRatio
	}
	if request.MaxRatio > 0 {
		payload["max_ratio"] = request.MaxRatio
	}
	if strings.TrimSpace(request.CallbackURL) != "" {
		payload["callback_url"] = request.CallbackURL
	}
	if request.ReturnURL {
		payload["return_url"] = request.ReturnURL
	}
	if strings.TrimSpace(request.LogoInfo) != "" {
		payload["logo_info"] = request.LogoInfo
	}
	if strings.TrimSpace(request.AIGCMeta) != "" {
		payload["aigc_meta"] = request.AIGCMeta
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return JimengImageResponse{}, errors.Wrap(err, "failed to marshal payload")
	}

	httpRequest, err := http.NewRequest("POST", apiURL, bytes.NewReader(data))
	if err != nil {
		return JimengImageResponse{}, errors.Wrap(err, "failed to create request")
	}

	// 添加签名
	if err := signJimengRequest(httpRequest, config, data); err != nil {
		return JimengImageResponse{}, errors.Wrap(err, "failed to sign request")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Do(httpRequest)
	if err != nil {
		return JimengImageResponse{}, errors.Wrap(err, "failed to send request")
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return JimengImageResponse{}, errors.Wrap(err, "failed to read response")
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return JimengImageResponse{}, errors.Errorf("API request failed with status %d: %s", response.StatusCode, truncate(string(body), 500))
	}

	var result JimengImageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return JimengImageResponse{}, errors.Wrap(err, "failed to unmarshal response")
	}

	if result.Code != 10000 {
		return JimengImageResponse{}, errors.Errorf("API error: code=%d, message=%s", result.Code, result.Message)
	}

	return result, nil
}

func buildJimengURL(baseURL string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsedURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	query := url.Values{}
	query.Set("Action", JimengAction)
	query.Set("Version", JimengVersion)
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}