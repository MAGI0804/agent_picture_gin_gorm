# Google 模型三 Key 配置说明

更新时间：2026-05-26

## 目标

Google 模型按能力拆成 3 个独立 API Key：

| 能力 | 默认模型 | API Key 用途 | 目的 |
| --- | --- | --- | --- |
| 文本 / 多模态规划 | `gemini-3.5-flash` | `GOOGLE_GEMINI_TEXT_API_KEY` | 规划、记忆、Prompt、文本评审 |
| 图片生成 | `imagen-4.0-ultra-generate-001` | `GOOGLE_IMAGEN_API_KEY` | 最高质量出图 |
| 视觉理解 | `gemini-3.5-flash` | `GOOGLE_GEMINI_VISION_API_KEY` | 后续真实 Vision/OCR Review |

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

注意：当前 V2 的 `vision_review_agent` 还是 mock。Vision Key 和 Vision 模型配置可以先落库，但真实图片理解调用需要后续新增 Vision provider，把 artifact 图片内容传给 Gemini 多模态接口。

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

## 后续接入建议

1. 文本和图片已可按上述两条默认配置运行。
2. Vision 配置先入库，后续新增真实 Vision provider 时按 `config_info.capability = 'vision'` 查找，避免误用文本 Key。
3. 后续接更多厂商时保持同样结构：`provider`、`api_type`、`capability`、`base_url`、`api_key`，把厂商差异留在 provider adapter 内部。
