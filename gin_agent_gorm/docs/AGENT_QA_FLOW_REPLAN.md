# Agent 问答流程重新规划

## 目标

在不改动已配置好的模型、接口基础能力和模型调用方法的前提下，重新编排用户输入、智能优化、智能问答、图片模型调用和右侧结果展示流程。

本次规划优先复用现有能力：

- `deepseek-v4-pro` 文本模型：用于提示词优化和智能追问。
- `/api/prompts/optimize`：现有智能优化接口。
- `/api/conversations/:id/messages`：现有消息提交接口。
- `follow_up_questions`、`agent_steps`、`artifacts`：现有过程、追问、产物数据结构。
- 右侧 `artifact-panel`：现有图片/产物展示区域。

## 当前基线

当前前端 `ChatView.vue` 已具备：

- 用户输入框。
- 任务模式选择：文本模式、图片模式。
- 模型选择。
- `智能优化` 按钮。
- 优化后提示词展示和二次选择按钮。
- 补充问题回答框。
- Agent 执行步骤展示。
- 右侧产物、消息、步骤 Tab。
- 图片缩略图和大图预览。

当前后端已具备：

- `OptimizePrompt`：调用 `deepseek-v4-pro` 优化提示词。
- `createClarifyingTurn`：生成追问问题。
- `executeGeneration`：调用图片模型并保存图片产物。
- `agent_steps`：记录 context、prompt、image、review、artifact 等步骤。
- `artifacts`：保存并返回生成图片。

主要需要调整的是流程触发时机、前端交互状态和过程展示文案，不优先重写底层模型请求方法。

## 新流程总览

用户输入后分为三种路径：

1. 普通发送：直接使用用户原始提示词进行问答或图片生成。
2. 智能优化：先调用 `deepseek-v4-pro` 优化提示词，显示优化过程和结果，由用户选择原提示词或优化提示词继续发送。
3. 智能问答：先调用 `deepseek-v4-pro` 生成针对性问题，用户回答后，将原始输入、问题、回答一起发送给图片模型。

图片模式需要额外规则：

- 当最终要发送给图片模型的提示词超过 750 字符时，自动触发智能优化。
- 自动优化后的提示词必须不超过 750 字符。
- 自动优化过程需要完整显示。
- 自动优化完成后不需要用户确认，直接用优化后提示词发送图片模型。

## 流程一：用户手动点击智能优化

### 触发条件

用户输入内容后点击 `智能优化` 按钮。

### 调用模型

固定调用默认 `deepseek-v4-pro`。

### 发送给模型的内容

```text
请将下面的提示词进行优化，优化后的提示词显示出来：

{用户原始提示词}
```

如果沿用现有 `/api/prompts/optimize`，后端需要保证其内部 prompt 语义与上面要求一致，且返回字段继续保持：

- `original_prompt`
- `optimized_prompt`
- `original_length`
- `optimized_length`
- `target_length`

### 前端展示

优化过程展示在输入区附近的过程面板中，至少包含：

- 原始提示词。
- 正在调用 `deepseek-v4-pro` 优化。
- 优化后的提示词。
- 原始长度和优化后长度。
- 操作按钮：
  - `使用原提示词`
  - `使用优化提示词`

### 用户选择后

- 选择 `使用原提示词`：提交原始 `normalText`。
- 选择 `使用优化提示词`：提交 `optimizedPromptText`，并设置：
  - `is_optimized=true`
  - `optimized_prompt={优化后的提示词}`

## 流程二：图片模式提示词超过 750 字符

### 触发条件

用户当前选择的是图片模型或图片模式，并且准备发送给图片模型的提示词长度超过 750 字符。

触发点包括：

- 用户直接点击发送。
- 用户点击 `使用原提示词`，但原提示词超过 750 字符。
- 智能问答合并后的最终提示词超过 750 字符。

### 处理规则

1. 前端检测最终提示词长度。
2. 超过 750 字符时，不弹确认，不阻断。
3. 自动调用 `/api/prompts/optimize`，`target_length=750`。
4. 后端使用 `deepseek-v4-pro` 缩短提示词。
5. 前端展示自动优化全过程。
6. 如果优化结果仍超过 750 字符，后端或前端需要再次截断/二次优化，直到不超过 750 字符。
7. 直接将不超过 750 字符的优化提示词发送图片模型。

### 前端过程展示

建议新增 `processTimeline` 状态，统一显示：

- `检测到图片提示词超过 750 字符`
- `正在调用 deepseek-v4-pro 自动优化`
- `优化完成：{original_length} -> {optimized_length}`
- `已使用优化后提示词提交图片模型`
- `图片模型生成中`
- `图片已生成，右侧显示`

### 后端兜底

即使前端已自动优化，后端仍需要兜底：

- `prepareGenerationPromptInput` 不应再直接返回“提示词太长”错误。
- 图片生成前若发现内容超过 750 字符，应自动调用 `optimizePromptWithDeepseek(..., 750, "shorten")`。
- `ensureImagePromptLength` 需要保证最终 prompt 不超过 750 字符。
- 如果 deepseek 优化失败，再返回明确错误：`图片提示词自动优化失败，请稍后重试`。

## 流程三：新增智能问答按钮

### 触发条件

用户点击新增按钮 `智能问答` 后发送内容。

推荐交互方式：

- 在输入区工具栏新增 `智能问答` 按钮。
- 点击后进入智能问答流程，不直接调用图片模型。

### 第一阶段：生成针对性问题

前端发送：

```json
{
  "input_type": "normal",
  "task_type": "image_generation",
  "content": "{用户原始输入}",
  "question_mode": "smart_qa",
  "text_model_config_id": "{deepseek-v4-pro 或当前文本模型 ID}"
}
```

如果暂不新增字段，也可以复用现有 `createClarifyingTurn`，但建议增加 `question_mode` 或 `workflow_mode`，避免和普通图片生成流程混淆。

后端调用 `deepseek-v4-pro` 输出针对性问题：

- 最多 3 个问题。
- 聚焦图片生成必要信息。
- 问题应该具体、可回答。
- 不输出泛泛而谈的问题。

示例问题方向：

- 图片用途、主体、风格。
- 尺寸比例。
- 必须出现或禁止出现的元素。
- 文案、Logo、颜色、布局。
- 商业产品图的拍摄角度和场景。

### 第二阶段：用户回答

用户回答后，前端将以下内容合并：

- 原始提示词。
- deepseek 生成的问题。
- 用户逐条回答。

推荐最终内容格式：

```text
原始需求：
{用户原始输入}

补充问题与回答：
1. {问题1}
回答：{回答1}
2. {问题2}
回答：{回答2}
3. {问题3}
回答：{回答3}
```

提交给图片模型前执行 750 字符检查：

- 不超过 750 字符：直接发送图片模型。
- 超过 750 字符：自动触发流程二。

### 第三阶段：发送图片模型

发送 `/api/conversations/:id/messages`：

```json
{
  "input_type": "answer_to_questions",
  "task_type": "image_generation",
  "content": "{合并后的内容或自动优化后的内容}",
  "image_model_config_id": "{当前图片模型 ID}",
  "text_model_config_id": "{deepseek-v4-pro 或当前文本模型 ID}",
  "answered_question_ids": ["..."],
  "is_optimized": true,
  "optimized_prompt": "{如果发生自动优化则填写}"
}
```

## 右侧展示规划

右侧需要显示图片，同时所有过程也要显示出来。

推荐右侧分区：

- 顶部：过程时间线。
- 中部：图片预览。
- 底部或 Tab：消息、步骤详情。

如果继续保留现有 Tab，建议默认行为调整为：

- 图片生成流程开始后，右侧默认选中 `产物`。
- `产物`页顶部显示本轮过程摘要。
- 图片生成成功后，图片出现在右侧主预览区域。
- `步骤`页显示完整 `agent_steps`。

右侧过程摘要需要覆盖：

- 用户原始输入。
- 是否点击智能优化。
- 是否自动触发 750 字符优化。
- 优化前后提示词。
- 是否进入智能问答。
- deepseek 生成的问题。
- 用户回答。
- 最终发送给图片模型的提示词。
- 图片模型返回状态。

## 前端改动点

建议在 `frontend/src/views/ChatView.vue` 中增加或调整：

- `smartQaMode`：是否进入智能问答流程。
- `processTimeline`：当前流程展示列表。
- `optimizationMode`：`manual` 或 `auto`。
- `originalPromptForCurrentRun`：本轮原始提示词。
- `finalPromptForImage`：最终发送图片模型的提示词。
- `smartQuestions`：智能问答生成的问题。
- `smartAnswers`：用户回答。

需要调整的函数：

- `optimizeNormalPrompt`
  - 手动优化时使用固定展示文案。
  - 结果显示后等待用户选择。
- `sendNormal`
  - 图片模式发送前执行 750 字符检查。
  - 超长时自动优化并直接提交。
- `sendAnswer`
  - 合并原始需求、问题、回答。
  - 图片发送前执行 750 字符检查。
- 新增 `startSmartQa`
  - 调用后端生成问题。
  - 展示问题并等待用户回答。
- 新增 `appendProcessStep`
  - 所有流程节点统一写入过程面板。

## 后端改动点

建议保持现有模型调用方法，只调整编排逻辑：

- `OptimizePrompt`
  - 明确使用 `deepseek-v4-pro`。
  - 支持 `target_length=750`。
  - 对自动优化场景强约束结果长度。
- `SendMessage`
  - 支持 `question_mode` 或 `workflow_mode`。
  - 智能问答模式下先走追问，不直接图片生成。
- `prepareGenerationPromptInput`
  - 图片提示词超过 750 字符时自动优化。
  - 不再直接要求用户手动优化。
- `ensureImagePromptLength`
  - 最终兜底，保证发送给图片模型的 prompt 不超过 750 字符。
- `agent_steps`
  - 新增或明确以下步骤：
    - `manual_prompt_optimize_agent`
    - `auto_prompt_optimize_agent`
    - `smart_question_agent`
    - `smart_answer_merge_agent`
    - `image_prompt_length_check_agent`

## 推荐接口扩展

为了避免破坏现有接口，可以只在请求体中新增可选字段：

```go
QuestionMode string `json:"question_mode" form:"question_mode"`
WorkflowMode string `json:"workflow_mode" form:"workflow_mode"`
OriginalPrompt string `json:"original_prompt" form:"original_prompt"`
ProcessTrace []ProcessTraceItem `json:"process_trace" form:"process_trace"`
```

最小实现可以先只新增：

```go
QuestionMode string `json:"question_mode" form:"question_mode"`
OriginalPrompt string `json:"original_prompt" form:"original_prompt"`
```

其中：

- `question_mode=smart_qa`：强制先生成针对性问题。
- `original_prompt`：智能问答回答阶段保留原始需求，便于合并和记录。

## 验收标准

### 手动智能优化

- 用户点击 `智能优化` 后，调用 `deepseek-v4-pro`。
- 页面显示原提示词、优化中、优化后提示词。
- 用户可以选择原提示词或优化提示词发送。
- 发送后消息和步骤能体现是否使用优化提示词。

### 图片模式超长自动优化

- 图片模式下，提示词超过 750 字符时自动优化。
- 优化后提示词不超过 750 字符。
- 不需要用户确认，自动提交图片模型。
- 页面完整显示自动优化过程。
- 后端不会再直接返回“提示词太长，请重新输入或使用智能优化”。

### 智能问答

- 页面有 `智能问答` 按钮。
- 点击后先生成针对性问题。
- 用户回答后，原始输入、问题、回答一起参与图片生成。
- 若合并后超过 750 字符，自动优化后再生成图片。
- 整个过程在页面上可见。

### 右侧展示

- 图片生成过程默认在右侧展示。
- 图片生成成功后，图片显示在右侧。
- 右侧能看到优化、追问、回答、最终提示词、图片生成状态等过程。

## 实施顺序

1. 前端先整理状态和过程展示，新增 `processTimeline`。
2. 调整手动智能优化展示和选择逻辑。
3. 增加图片模式 750 字符自动优化流程。
4. 增加 `智能问答` 按钮和问题回答 UI。
5. 后端增加智能问答模式字段和流程分支。
6. 后端为图片超长提示词增加自动优化兜底。
7. 联调三条主流程。
8. 验证右侧图片和过程展示。

