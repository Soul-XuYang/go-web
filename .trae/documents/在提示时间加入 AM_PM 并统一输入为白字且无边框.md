## 目标
- 提示时间显示为 12 小时制并附带 AM/PM（示例：[03:42:50 PM]）。
- 输入文本颜色改为纯白；移除任何边框/阴影/包裹效果，避免出现白色输入框视觉。
- 保持现有 Enter/Ctrl+C/Ctrl+L、清屏与闪烁光标行为。

## 修改点
- templates/terminal.html：将 `#prompt-input` 的类从 `content content--command` 改为 `content`，避免黄色命令色覆盖输入文本颜色。
- static/js/terminal.js：
  - 更新时间格式：`formatHHMMSS` 改为 12 小时制，`updatePromptPrefix` 将 `[HH:MM:SS AM/PM]` 写入 `#prompt-ts`。
- static/terminal.css：
  - `.prompt-input` 强制白字：`color: #ffffff !important;`。
  - `.prompt-input:focus, .prompt-input:focus-visible` 取消 `outline/border/box-shadow/background`，确保无白色输入框视觉；保留闪烁光标逻辑。

## 验证
- 提示前缀显示如 `[03:42:50 PM] superadmin>`，每秒更新；用户名不变。
- 输入文本为纯白，聚焦时无白色边框或包裹效果。
- Enter/Ctrl+C/Ctrl+L 与清屏仍正常。