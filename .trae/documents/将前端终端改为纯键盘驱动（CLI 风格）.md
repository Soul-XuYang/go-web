## 现状确认
- Enter 执行：`static/js/terminal.js:234-239` 已在 `#command-input` 上监听 Enter 并调用 `sendCommand()`（排除 `Shift+Enter`）。
- 按钮执行：`static/js/terminal.js:233` 仍绑定 `#send-btn` 点击事件；本次不改动按钮可见性，仅确保键盘交互可用。
- 后端支持：`controllers/terminal_ws.go:413-432` 已处理斜杠命令（含 `/stop`）；`controllers/terminal_ws.go:433-440` 后端可下发 `type:"clear"` 让前端清屏。
- 清屏处理：前端消息处理已包含 `type:'clear'` 分支（位于 `static/js/terminal.js` 的 `handleIncomingMessage`；参考 `static/js/terminal.js:91-116`）。

## 目标与范围
- 保持 Enter 发送命令不变。
- 新增 Ctrl+C：在输入框按下时发送 `/stop` 到后端，并在输出区追加 `^C` 行。
- 新增 Ctrl+L：本地立即清屏；同时继续正确响应后端消息 `type:'clear'`。
- 不引入第三方库，不改后端。

## 前端改动点
- `static/js/terminal.js`
  - 扩展 `#command-input` 的 `keydown` 监听（就在现有 Enter 分支旁）：
    - `if (event.ctrlKey && event.key === 'c') {` 发送 `JSON.stringify({ command: '/stop', args: [] })`；随后 `appendLine({ type: 'stdout', data: '^C' })`；`event.preventDefault()`。
    - `if (event.ctrlKey && event.key === 'l') {` 调用本地 `clearOutput()` 清空输出容器；`event.preventDefault()`。
  - 复用现有 `sendCommand()`（`static/js/terminal.js:153-171`）与 WebSocket 链路（`static/js/terminal.js:173-215`）。
- `static/terminal.css`
  - 无必须改动；若需，确保清屏后滚动状态正常（输出容器 `overflow` 与滚动条样式保持）。

## 验证
- 进入 `/admin/superadmin/terminal` 页面（入口见 `templates/dashboard.html`）。
- 输入命令按 Enter 执行，行为与当前一致。
- 运行长命令后按 Ctrl+C：后端终止（`controllers/terminal_ws.go:413-432`），前端输出出现 `^C`；不会插入多余字符。
- 按 Ctrl+L：输出立刻清空；当后端主动返回 `type:'clear'` 时也正常清屏。
- 检查在普通与超管路径下 WebSocket 均正常（`static/js/terminal.js:20-26` 地址构建）。

## 风险与回滚
- 改动仅增加键盘分支，风险低；若需回滚，移除新增 `keydown` 分支即可。
- 不影响现有按钮执行逻辑与“按行输出”模式切换。