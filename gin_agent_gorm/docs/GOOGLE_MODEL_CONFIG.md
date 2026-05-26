# Google 模型三 Key 配置说明

更新时间：2026-05-26

## 目标

Google 模型按能力拆成 3 个独立 API Key：

| 能力 | 默认模型 | API Key 用途 | 目的 |
| --- | --- | --- | --- |
| 文本 / 多模态规划 | `gemini-3.5-flash` | `GOOGLE_GEMINI_TEXT_API_KEY` | 规划、记忆、Prompt、文本评审 |
| 图片生成 | `imagen-4.0-ultra-generate-001` | `GOOGLE_IMAGEN_API_KEY` | 最高质量出图 |
| 视觉理解 | `gemini-3.5-flash` | `GOOGLE_GEMINI_VISION_API_KEY` | 真实 Vision Review / 后续 OCR 增强 |

这样某一个能力触发限流、欠费、Key 失效或风控时，不会直接影响其他能力。

参考 Google 官方文档：

- Gemini OpenAI-compatible API：https://ai.google.dev/gemini-api/docs/openai
- Imagen 图片生成：https://ai.google.dev/gemini-api/docs/imagen
- Gemini 模型列表：https://ai.google.dev/gemini-api/docs/models

## 数据库存放位置

数据库连接配置文件：

```text
gin_agent_gorm/etc/config.yaml
```

使用其中的：

```text
DB.Driver
DB.Host
DB.Port
DB.Database
DB.Username
DB.Password
```

模型配置存放表：

```text
model_configs
```

关键字段：

```text
model_configs.model_name
model_configs.request_url
model_configs.is_text_model
model_configs.is_image_model
model_configs.support_thinking
model_configs.config_info
```

API Key 存放在：

```text
model_configs.config_info.api_key
```

用户默认模型选择存放在：

```text
user_model_configs.selected_text_model_config_id
user_model_configs.selected_image_model_config_id
```

用户模型权限存放在：

```text
user_model_permissions.user_id
user_model_permissions.model_config_id
user_model_permissions.can_use
```

## 推荐配置约定

### 1. Gemini 文本 / 多模态模型

用途：规划、记忆、Prompt 生成、文本评审。

```json
{
  "provider": "google",
  "api_type": "openai_compatible",
  "capability": "text",
  "base_url": "https://generativelanguage.googleapis.com/v1beta/openai",
  "api_key": "<GOOGLE_GEMINI_TEXT_API_KEY>",
  "temperature": "0.7"
}
```

运行时接口：

```text
POST https://generativelanguage.googleapis.com/v1beta/openai/chat/completions
Authorization: Bearer <GOOGLE_GEMINI_TEXT_API_KEY>
```

### 2. Imagen 图片生成模型

用途：最高质量出图。

```json
{
  "provider": "google",
  "api_type": "google_imagen",
  "capability": "image_generation",
  "base_url": "https://generativelanguage.googleapis.com/v1beta",
  "api_key": "<GOOGLE_IMAGEN_API_KEY>",
  "aspect_ratio": "1:1"
}
```

运行时接口：

```text
POST https://generativelanguage.googleapis.com/v1beta/models/imagen-4.0-ultra-generate-001:predict
x-goog-api-key: <GOOGLE_IMAGEN_API_KEY>
```

当前代码已支持该分支：`HTTPProvider.Generate` 会识别 `provider=google` 且 `model_name` 包含 `imagen`，然后走 Google Imagen 原生 `:predict`。

### 3. Gemini Vision 视觉理解模型

用途：后续真实 Vision/OCR Review，包括图片质量评审、图文一致性、OCR、低分原因提取。

```json
{
  "provider": "google",
  "api_type": "openai_compatible",
  "capability": "vision",
  "base_url": "https://generativelanguage.googleapis.com/v1beta/openai",
  "api_key": "<GOOGLE_GEMINI_VISION_API_KEY>",
  "temperature": "0.2"
}
```

运行时接口：

```text
POST https://generativelanguage.googleapis.com/v1beta/openai/chat/completions
Authorization: Bearer <GOOGLE_GEMINI_VISION_API_KEY>
```

注意：当前 V2 已接入真实 Google Vision provider。存在 `config_info.capability = 'vision'` 的可用 Google 文本模型配置时，workflow 会用 Gemini 多模态接口分析 artifact 图片并写入 review 分数；没有可用 Vision 配置时回退 mock review。

## 初始化 SQL

执行前替换 3 个 API Key 和 `@user_id`。

```sql
SET @user_id = 1;
SET @google_gemini_text_api_key = '<GOOGLE_GEMINI_TEXT_API_KEY>';
SET @google_imagen_api_key = '<GOOGLE_IMAGEN_API_KEY>';
SET @google_gemini_vision_api_key = '<GOOGLE_GEMINI_VISION_API_KEY>';

INSERT INTO model_configs
(model_name, request_url, is_text_model, is_image_model, support_thinking, config_info, created_at, updated_at)
VALUES
(
  'gemini-3.5-flash',
  'https://generativelanguage.googleapis.com/v1beta/openai',
  1,
  0,
  1,
  JSON_OBJECT(
    'provider', 'google',
    'api_type', 'openai_compatible',
    'capability', 'text',
    'base_url', 'https://generativelanguage.googleapis.com/v1beta/openai',
    'api_key', @google_gemini_text_api_key,
    'temperature', '0.7'
  ),
  UNIX_TIMESTAMP(),
  UNIX_TIMESTAMP()
);
SET @text_model_config_id = LAST_INSERT_ID();

INSERT INTO model_configs
(model_name, request_url, is_text_model, is_image_model, support_thinking, config_info, created_at, updated_at)
VALUES
(
  'imagen-4.0-ultra-generate-001',
  'https://generativelanguage.googleapis.com/v1beta',
  0,
  1,
  0,
  JSON_OBJECT(
    'provider', 'google',
    'api_type', 'google_imagen',
    'capability', 'image_generation',
    'base_url', 'https://generativelanguage.googleapis.com/v1beta',
    'api_key', @google_imagen_api_key,
    'aspect_ratio', '1:1'
  ),
  UNIX_TIMESTAMP(),
  UNIX_TIMESTAMP()
);
SET @image_model_config_id = LAST_INSERT_ID();

INSERT INTO model_configs
(model_name, request_url, is_text_model, is_image_model, support_thinking, config_info, created_at, updated_at)
VALUES
(
  'gemini-3.5-flash',
  'https://generativelanguage.googleapis.com/v1beta/openai',
  1,
  0,
  1,
  JSON_OBJECT(
    'provider', 'google',
    'api_type', 'openai_compatible',
    'capability', 'vision',
    'base_url', 'https://generativelanguage.googleapis.com/v1beta/openai',
    'api_key', @google_gemini_vision_api_key,
    'temperature', '0.2'
  ),
  UNIX_TIMESTAMP(),
  UNIX_TIMESTAMP()
);
SET @vision_model_config_id = LAST_INSERT_ID();

INSERT INTO user_model_configs
(
  user_id,
  selected_text_model_config_id,
  selected_image_model_config_id,
  provider,
  chat_model,
  image_model,
  base_url,
  api_key,
  temperature,
  created_at,
  updated_at
)
VALUES
(
  @user_id,
  @text_model_config_id,
  @image_model_config_id,
  'google',
  'gemini-3.5-flash',
  'imagen-4.0-ultra-generate-001',
  'https://generativelanguage.googleapis.com/v1beta/openai',
  '',
  '0.7',
  UNIX_TIMESTAMP(),
  UNIX_TIMESTAMP()
)
ON DUPLICATE KEY UPDATE
  selected_text_model_config_id = VALUES(selected_text_model_config_id),
  selected_image_model_config_id = VALUES(selected_image_model_config_id),
  provider = VALUES(provider),
  chat_model = VALUES(chat_model),
  image_model = VALUES(image_model),
  base_url = VALUES(base_url),
  temperature = VALUES(temperature),
  updated_at = UNIX_TIMESTAMP();

INSERT INTO user_model_permissions
(user_id, model_config_id, can_use, created_at, updated_at)
VALUES
(@user_id, @text_model_config_id, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
(@user_id, @image_model_config_id, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
(@user_id, @vision_model_config_id, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());
```

## 验证 SQL

```sql
SELECT
  id,
  model_name,
  is_text_model,
  is_image_model,
  support_thinking,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.provider')) AS provider,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.api_type')) AS api_type,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.capability')) AS capability,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.base_url')) AS base_url,
  CASE
    WHEN JSON_EXTRACT(config_info, '$.api_key') IS NULL THEN 'missing'
    ELSE 'present'
  END AS api_key_status
FROM model_configs
WHERE JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.provider')) = 'google'
ORDER BY id DESC;
```

```sql
SELECT
  user_id,
  selected_text_model_config_id,
  selected_image_model_config_id,
  provider,
  chat_model,
  image_model
FROM user_model_configs
WHERE user_id = @user_id;
```

## 替换正式 API Key

如果 E2E 输出了：

```text
google e2e target user_id=1 text_model_config_id=10 image_model_config_id=11
```

优先按具体 `id` 精确更新，避免误改历史配置。

```sql
SET @text_model_config_id = 10;
SET @image_model_config_id = 11;
SET @vision_model_config_id = 12;

SET @google_gemini_text_api_key = '<正式 GOOGLE_GEMINI_TEXT_API_KEY>';
SET @google_imagen_api_key = '<正式 GOOGLE_IMAGEN_API_KEY>';
SET @google_gemini_vision_api_key = '<正式 GOOGLE_GEMINI_VISION_API_KEY>';

UPDATE model_configs
SET
  config_info = JSON_SET(config_info, '$.api_key', @google_gemini_text_api_key),
  updated_at = UNIX_TIMESTAMP()
WHERE id = @text_model_config_id;

UPDATE model_configs
SET
  config_info = JSON_SET(config_info, '$.api_key', @google_imagen_api_key),
  updated_at = UNIX_TIMESTAMP()
WHERE id = @image_model_config_id;

UPDATE model_configs
SET
  config_info = JSON_SET(config_info, '$.api_key', @google_gemini_vision_api_key),
  updated_at = UNIX_TIMESTAMP()
WHERE id = @vision_model_config_id;
```

替换后执行：

```bash
go test -tags googlee2e ./internal/service/agent_v2/app -run TestGoogleModelEndToEnd -v
```

当前 E2E 已确认选中的配置为：

```text
user_id=1
text_model_config_id=5
image_model_config_id=6
```

如果 Google 返回 `API_KEY_INVALID`，优先替换图片生成配置 `id=6`：

```sql
SET @google_imagen_api_key = '<正式 GOOGLE_IMAGEN_API_KEY>';

UPDATE model_configs
SET
  config_info = JSON_SET(config_info, '$.api_key', @google_imagen_api_key),
  updated_at = UNIX_TIMESTAMP()
WHERE id = 6;
```

确认不要打印明文 Key，只检查是否已写入：

```sql
SELECT
  id,
  model_name,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.provider')) AS provider,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.api_type')) AS api_type,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.capability')) AS capability,
  LENGTH(JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.api_key'))) AS api_key_length
FROM model_configs
WHERE id IN (5, 6);
```

如果 E2E 仍返回 `API_KEY_INVALID`，先确认数据库里读取到的 Key 摘要已经变化。E2E 日志会输出：

```text
api_key_length=...
api_key_sha256=...
```

也可以用 SQL 对照，不打印明文 Key：

```sql
SELECT
  id,
  model_name,
  request_url,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.api_type')) AS api_type,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.base_url')) AS base_url,
  JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.capability')) AS capability,
  LENGTH(JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.api_key'))) AS api_key_length,
  LEFT(SHA2(JSON_UNQUOTE(JSON_EXTRACT(config_info, '$.api_key')), 256), 12) AS api_key_sha256
FROM model_configs
WHERE id = 6;
```

`API_KEY_INVALID` 通常只剩这几类原因：

- `model_configs.id=6` 没有真正更新，E2E 仍读到旧 Key。
- Key 复制时包含空格、引号、换行，或仍是占位符。
- Key 不是 Google AI Studio / Gemini API Key，或该 Key 无权访问 `generativelanguage.googleapis.com`。
- Google Cloud 控制台限制了 API Key 的可用 API 或来源，导致 Generative Language API 不可用。

## 后续接入建议

当前真实后端 E2E 已通过：

```text
go test -tags googlee2e ./internal/service/agent_v2/app -run TestGoogleModelEndToEnd -v
```

验收结果：

```text
user_id=1
text_model_config_id=5
image_model_config_id=6
conversation_id=35
run_id=48
artifact_id=27
version_id=1
provider=google
image_model=imagen-4.0-ultra-generate-001
bytes=971755
preview_url=/api/v2/artifacts/27/preview
```

后续建议：

1. 文本和图片已可按上述两条默认配置运行。
2. Vision 配置按 `config_info.capability = 'vision'` 查找，避免误用文本 Key；真实 review 已接入，复杂 OCR/版面检测可作为后续增强。
3. 后续接更多厂商时保持同样结构：`provider`、`api_type`、`capability`、`base_url`、`api_key`，把厂商差异留在 provider adapter 内部。
