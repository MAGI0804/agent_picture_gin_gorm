package middleware

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/helper/strx"
	"gin-biz-web-api/pkg/logger"
)

type AccessLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write 在此方法中实现了双写，因此可以直接通过 `AccessLogWriter.body` 获取到方法返回的响应主体
func (w AccessLogWriter) Write(p []byte) (int, error) {
	if n, err := w.body.Write(p); err != nil {
		return n, err
	}

	return w.ResponseWriter.Write(p)
}

// AccessLog 记录请求日志
// 参考：gin.Logger()
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {

		// 如果是访问静态资源或 artifacts，不记录响应体
		requestPath := c.Request.URL.Path
		skipResponseBody := strings.HasPrefix(requestPath, config.GetString("cfg.upload.static_fs_relative_path")) ||
			strings.HasPrefix(requestPath, "/artifacts/") ||
			(strings.HasPrefix(requestPath, "/api/v2/artifacts/") &&
				(strings.HasSuffix(requestPath, "/preview") || strings.HasSuffix(requestPath, "/download")))

		// 获取 response 内容
		var responseBodyWriter *AccessLogWriter
		if !skipResponseBody {
			responseBodyWriter = &AccessLogWriter{
				ResponseWriter: c.Writer,
				body:           bytes.NewBufferString(""),
			}
			c.Writer = responseBodyWriter
		}

		// 获取请求数据
		var requestBody []byte
		if c.Request.Body != nil {
			// c.Request.Body 是一个 buffer 对象，只能读取一次
			requestBody, _ = ioutil.ReadAll(c.Request.Body)
			// 读取后，重新赋值 c.Request.Body ，以供后续的其他操作
			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 设置开始时间
		start := time.Now()
		c.Next()
		// 程序执行花费时间
		cost := time.Since(start)

		// http 响应状态码
		var responseStatus int
		if skipResponseBody {
			responseStatus = c.Writer.Status()
		} else {
			responseStatus = responseBodyWriter.Status()
		}

		// 开始记录日志
		logFields := []zap.Field{
			zap.String("request_method", c.Request.Method),                               // 当前请求的方法
			zap.String("request_url", sanitizedURLString(c.Request.Host, c.Request.URL)), // 完整的请求地址（host + path + query）eg：`0.0.0.0:3000/api/user?aa=11&bb=22`
			zap.String("request_path", c.Request.URL.Path),                               // 只有请求地址，不带参数 eg：`/api/user`
			zap.String("request_uri", sanitizedRequestURI(c.Request.URL)),                // 带参数的地址 eg： `/api/user?aa=11&bb=22`
			zap.String("request_query", sanitizedRawQuery(c.Request.URL.RawQuery)),       // 只有参数 eg：`aa=11&bb=22`
			zap.String("client_ip", c.ClientIP()),                                        // 客户端的 ip 地址
			zap.String("remote_addr", c.Request.RemoteAddr),
			zap.String("user_agent", c.Request.UserAgent()),        // 用户请求头
			zap.Any("headers", sanitizedHeaders(c.Request.Header)), // 请求头
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Int("response_status", responseStatus),                  // 当前的响应结果状态码
			zap.String("code_execute_time", strx.StrMicroseconds(cost)), // 程序执行时间
		}

		// 记录请求体内容 eg：`"x=33&y=zz"`
		var logRequestBody string
		if "multipart/form-data" == c.ContentType() {
			// 上传文件时，不会记录上传文件资源数据
			logRequestBody = c.Request.PostForm.Encode()
		} else {
			logRequestBody, _ = url.QueryUnescape(string(requestBody)) // 中文会被加码，因此为了方便查看中文参数，对请求体进行解码
		}
		logFields = append(logFields, zap.String("request_body", logRequestBody))

		// 响应的内容 - 只在非静态资源和非 artifacts 路径记录
		var logResponseBody string
		if skipResponseBody {
			logResponseBody = "[binary content skipped]"
		} else {
			logResponseBody = responseBodyWriter.body.String()
		}
		logFields = append(logFields, zap.String("response_body", logResponseBody))

		// 记录访问日志
		logger.Info("HTTP Access Log [ "+cast.ToString(responseStatus)+" ]", logFields...)

	}
}

func sanitizedHeaders(headers http.Header) map[string][]string {
	result := make(map[string][]string, len(headers))
	for key, values := range headers {
		if isSensitiveHeader(key) {
			result[key] = []string{"[REDACTED]"}
			continue
		}
		copied := make([]string, len(values))
		copy(copied, values)
		result[key] = copied
	}
	return result
}

func isSensitiveHeader(key string) bool {
	switch strings.ToLower(key) {
	case "authorization", "cookie", "set-cookie", "token", "x-api-key", "api_key", "apikey", "access_token":
		return true
	default:
		return false
	}
}

func sanitizedURLString(host string, requestURL *url.URL) string {
	if requestURL == nil {
		return host
	}
	cloned := *requestURL
	cloned.RawQuery = sanitizedRawQuery(requestURL.RawQuery)
	return host + cloned.String()
}

func sanitizedRequestURI(requestURL *url.URL) string {
	if requestURL == nil {
		return ""
	}
	cloned := *requestURL
	cloned.RawQuery = sanitizedRawQuery(requestURL.RawQuery)
	return cloned.RequestURI()
}

func sanitizedRawQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	for key := range values {
		if isSensitiveHeader(key) {
			values.Set(key, "[REDACTED]")
		}
	}
	return values.Encode()
}
