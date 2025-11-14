// 单独的终端页面JS脚本
'use strict';
(function () {
    const outputEl = document.getElementById('output');
    const promptLineEl = document.getElementById('prompt-line');
    const promptInputEl = document.getElementById('prompt-input');
    const promptPrefixEl = document.getElementById('prompt-prefix');
    const promptTsEl = document.getElementById('prompt-ts');
    const promptUserEl = document.getElementById('prompt-user');
    const legacyInputEl = document.getElementById('command-input');
    const inputEl = promptInputEl || legacyInputEl;
    const buttonEl = document.getElementById('send-btn');
    const statusEl = document.getElementById('status');
    const infoEl = document.getElementById('terminal-info');
    const jumpBtn = document.getElementById('open-terminal-page');
    const lineModeBtn = document.getElementById('toggle-line-mode');

    if (!outputEl || !inputEl || !statusEl) {
        console.error('Terminal elements missing');
        return;
    }

    const { pathname } = window.location;
    const isSuperadminContext = pathname.startsWith('/api/superadmin') || pathname.startsWith('/admin/superadmin');

    const wsUrl = (() => {
        const { protocol, host } = window.location;
        const base = isSuperadminContext ? '/api/superadmin/terminal' : '/api/ws/terminal';
        const wsProtocol = protocol === 'https:' ? 'wss:' : 'ws:';
        return `${wsProtocol}//${host}${base}`;
    })();
    const infoUrl = isSuperadminContext ? '/api/superadmin/terminal/info' : '/api/ws/terminal/info';

    let socket = null;
    let reconnectAttempts = 0;
    const maxReconnect = 5;
    let infoLoaded = false;
    let userLabel = isSuperadminContext ? 'superadmin' : 'stdout';
    let lineModeEnabled = false;

    function setStatus(state, text) {
        statusEl.classList.remove('status--idle', 'status--connected', 'status--error');
        statusEl.classList.add(`status--${state}`);
        statusEl.textContent = text;
    }

    function formatHHMMSS(dateInput = new Date()) {
        const date = dateInput instanceof Date ? dateInput : new Date(dateInput);
        const two = (n) => String(n).padStart(2, '0');
        return `${two(date.getHours())}:${two(date.getMinutes())}:${two(date.getSeconds())}`;
    }

    function updatePromptPrefix() {
        if (promptTsEl) promptTsEl.textContent = `[${formatHHMMSS()}]`;
        if (promptUserEl) promptUserEl.textContent = `${userLabel}>`;
    }

    function appendLine(message) {
        if (message.type === 'clear') {
            const children = Array.from(outputEl.childNodes);
            for (const node of children) {
                if (promptLineEl && node === promptLineEl) continue;
                outputEl.removeChild(node);
            }
            return;
        }
        const line = document.createElement('div');
        line.className = `line ${message.type || 'stdout'}`;

        const prefix = document.createElement('span');
        prefix.className = 'prefix';
        const timestamp = formatHHMMSS(message.timestamp || Date.now());
        const type = message.type || 'stdout';
        const label = (() => {
            if (type === 'command') return `${userLabel}>`;
            if (type === 'stdout' || type === 'status' || type === 'time') return 'Terminal>';
            if (type === 'stderr' || type === 'error') return 'error>';
            return type;
        })();
        const timeSpan = document.createElement('span');
        timeSpan.className = 'timestamp';
        timeSpan.textContent = `[${timestamp}] `;

        const labelSpan = document.createElement('span');
        labelSpan.className = 'label';
        labelSpan.textContent = label;

        prefix.appendChild(timeSpan);
        prefix.appendChild(labelSpan);

        const content = document.createElement('span');
        content.className = `content content--${type}`;
        if (lineModeEnabled && typeof message.data === 'string') {
            const escape = (segment) => segment
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;');
            content.innerHTML = message.data
                .split(/\r?\n/)
                .map(segment => segment === '' ? '&nbsp;' : escape(segment))
                .join('<br />');
        } else {
            content.textContent = message.data ?? '';
        }

        line.appendChild(prefix);
        line.appendChild(content);
        if (promptLineEl) {
            outputEl.insertBefore(line, promptLineEl);
        } else {
            outputEl.appendChild(line);
        }
        outputEl.scrollTop = outputEl.scrollHeight;
    }

    function renderTerminalInfo(info) {
        if (!infoEl) return;
        const rows = [
            ['服务器系统', info?.os || '--'],
            ['服务器架构', info?.architecture || '--'],
            ['Go 版本', info?.go_version || '--'],
            ['当前用户', info?.username || '--']
        ];
        userLabel = info?.username || userLabel;
        infoEl.classList.remove('terminal-info--error');
        infoEl.innerHTML = rows.map(([label, value]) => `<span><strong>${label}：</strong>${value}</span>`).join('');
        updatePromptPrefix();
    }

    async function loadTerminalInfo() {
        if (!infoEl) return;
        try {
            infoEl.textContent = '环境信息加载中…';
            const res = await fetch(infoUrl, { credentials: 'include' });
            if (!res.ok) throw new Error(res.statusText);
            const data = await res.json();
            renderTerminalInfo(data);
        } catch (err) {
            infoEl.textContent = '环境信息加载失败';
            infoEl.classList.add('terminal-info--error');
        }
    }

    function handleIncomingMessage(message) {
        const type = message.type || 'stdout';
        switch (type) {
            case 'ready': {
                setStatus('connected', '通道已就绪');
                appendLine({ ...message, type: 'status' });
                if (!infoLoaded) {
                    infoLoaded = true;
                    loadTerminalInfo();
                }
                break;
            }
            case 'time': {
                const ts = formatHHMMSS(message.timestamp || Date.now());
                setStatus('connected', `心率 ${ts}`);
                updatePromptPrefix();
                break;
            }
            case 'clear': {
                appendLine(message);
                break;
            }
            case 'status': {
                setStatus('connected', message.data || '状态更新');
                break;
            }
            case 'error': {
                setStatus('error', '命令执行异常');
                appendLine(message);
                break;
            }
            default:
                appendLine(message);
        }
    }

    function sendCommand() {
        const rawSource = inputEl.tagName === 'INPUT' ? inputEl.value : inputEl.textContent;
        const raw = (rawSource || '').trim();
        if (!raw || !socket || socket.readyState !== WebSocket.OPEN) {
            return;
        }

        const [command, ...args] = raw.split(/\s+/g);
        appendLine({
            type: 'command',
            data: raw,
            timestamp: new Date().toISOString()
        });

        if (command === 'clear') {
            // 后端会再发 clear 消息，此处让用户指令保留，实际清屏交给 appendLine 处理
        }
        socket.send(JSON.stringify({ command, args, lineChoice: lineModeEnabled }));
        if (inputEl.tagName === 'INPUT') {
            inputEl.value = '';
        } else {
            inputEl.textContent = '';
        }
    }

    function connect() {
        setStatus('idle', '连接中…');
        socket = new WebSocket(wsUrl);

        socket.addEventListener('open', () => {
            reconnectAttempts = 0;
            setStatus('connected', '已连接');
            appendLine({
                type: 'status',
                data: `WebSocket connected (${wsUrl})`,
                timestamp: new Date().toISOString()
            });
            inputEl.focus();
        });

        socket.addEventListener('message', (event) => {
            try {
                const msg = JSON.parse(event.data);
                handleIncomingMessage(msg);
            } catch (err) {
                appendLine({
                    type: 'stderr',
                    data: `无法解析消息: ${event.data}`,
                    timestamp: new Date().toISOString()
                });
            }
        });

        socket.addEventListener('close', () => {
            setStatus('idle', '已断开');
            appendLine({
                type: 'status',
                data: 'WebSocket disconnected',
                timestamp: new Date().toISOString()
            });
            attemptReconnect();
        });

        socket.addEventListener('error', (event) => {
            console.error('WebSocket error', event);
            setStatus('error', '连接异常');
        });
    }

    function attemptReconnect() {
        if (reconnectAttempts >= maxReconnect) {
            appendLine({
                type: 'error',
                data: 'Too many reconnections, the attempt has been stopped',
                timestamp: new Date().toISOString()
            });
            setStatus('error', '已停止重连');
            return;
        }

        reconnectAttempts += 1;
        const delay = Math.min(5000, 1000 * reconnectAttempts);
        setTimeout(connect, delay);
    }

    if (buttonEl) {
        buttonEl.addEventListener('click', sendCommand);
    }
    inputEl.addEventListener('keydown', (event) => {
        // Enter 执行命令（排除 Shift+Enter）
        if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            sendCommand();
            return;
        }

        // Ctrl+C 发送 /stop，并在输出区追加 ^C
        if (event.ctrlKey && (event.key === 'c' || event.key === 'C')) {
            event.preventDefault();
            if (socket && socket.readyState === WebSocket.OPEN) {
                socket.send(JSON.stringify({ command: '/stop', args: [], lineChoice: lineModeEnabled }));
                appendLine({ type: 'stdout', data: '^C', timestamp: new Date().toISOString() });
            }
            return;
        }

        // Ctrl+L 本地清屏（同时保留后端 clear 的处理）
        if (event.ctrlKey && (event.key === 'l' || event.key === 'L')) {
            event.preventDefault();
            appendLine({ type: 'clear' });
            return;
        }
    });

    if (jumpBtn) {
        jumpBtn.addEventListener('click', () => {
            window.location.href = '/admin/dashboard';
        });
    }

    if (lineModeBtn) {
        lineModeBtn.addEventListener('click', () => {
            lineModeEnabled = !lineModeEnabled;
            lineModeBtn.classList.toggle('is-active', lineModeEnabled);
            lineModeBtn.querySelector('.icon').textContent = lineModeEnabled ? '☑' : '☐';
            lineModeBtn.querySelector('.label').textContent = lineModeEnabled ? '按行输出（开）' : '按行输出（关）';
        });
    }

    // 支持点击输出区域后使用快捷键（如 Ctrl+C/Ctrl+F）
    outputEl.addEventListener('click', () => {
        inputEl.focus();
    });

    function setupPromptCursor() {
        const set = (focused) => inputEl.setAttribute('data-focused', focused ? 'true' : 'false');
        set(document.activeElement === inputEl);
        inputEl.addEventListener('focus', () => set(true));
        inputEl.addEventListener('blur', () => set(false));
    }
    setupPromptCursor();

    setInterval(updatePromptPrefix, 1000);

    // 防止 contenteditable 换行与粘贴带段落
    inputEl.addEventListener('beforeinput', (event) => {
        if (event.inputType === 'insertParagraph') {
            event.preventDefault();
        }
    });
    inputEl.addEventListener('paste', (event) => {
        event.preventDefault();
        const text = (event.clipboardData || window.clipboardData).getData('text');
        const clean = String(text || '').replace(/\r?\n/g, ' ');
        document.execCommand('insertText', false, clean);
    });

    connect();
})();


