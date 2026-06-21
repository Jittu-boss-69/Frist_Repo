// DevSync Client Application Logic

const STATE = {
  token: localStorage.getItem('devsync_token') || '',
  user: JSON.parse(localStorage.getItem('devsync_user') || 'null'),
  activeTab: 'dashboard',
  editor: null,
  selectedSnippet: null,
  snippetsList: [],
  filesList: [],
  devicesList: [],
  activitiesList: [],
  ws: null,
  localIP: '127.0.0.1',
  port: '8080',
};

// ==========================================
// TOAST NOTIFICATIONS
// ==========================================
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container');
  const toast = document.createElement('div');
  toast.className = `toast-notification glass-panel px-5 py-3 rounded-xl border flex items-center gap-3 text-sm shadow-xl z-50`;
  
  let icon = '<i class="fa-solid fa-circle-info text-blue-400"></i>';
  if (type === 'success') {
    icon = '<i class="fa-solid fa-circle-check text-green-400"></i>';
    toast.classList.add('border-green-500/20', 'glow-green');
  } else if (type === 'error') {
    icon = '<i class="fa-solid fa-triangle-exclamation text-red-400"></i>';
    toast.classList.add('border-red-500/20');
  } else {
    toast.classList.add('border-blue-500/20', 'glow-blue');
  }

  toast.innerHTML = `
    ${icon}
    <div class="flex-1 font-medium">${message}</div>
    <button class="text-slate-500 hover:text-slate-300" onclick="this.parentElement.remove()">
      <i class="fa-solid fa-xmark"></i>
    </button>
  `;

  container.appendChild(toast);
  setTimeout(() => {
    toast.style.animation = 'slide-in 0.3s cubic-bezier(0.16, 1, 0.3, 1) reverse';
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

// ==========================================
// UTILITY FUNCTIONS
// ==========================================
function copyText(elementId) {
  const element = document.getElementById(elementId);
  const text = element.innerText || element.value;
  navigator.clipboard.writeText(text).then(() => {
    showToast('Copied to clipboard!', 'success');
  }).catch(() => {
    showToast('Failed to copy', 'error');
  });
}

function formatBytes(bytes) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// ==========================================
// API REQUEST HELPER
// ==========================================
async function apiRequest(endpoint, method = 'GET', body = null, isMultipart = false) {
  const headers = {};
  if (STATE.token) {
    headers['Authorization'] = `Bearer ${STATE.token}`;
  }
  if (!isMultipart && body) {
    headers['Content-Type'] = 'application/json';
  }

  const options = {
    method,
    headers,
  };

  if (body) {
    options.body = isMultipart ? body : JSON.stringify(body);
  }

  try {
    const response = await fetch(endpoint, options);
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || 'Request failed');
    }
    return data;
  } catch (error) {
    console.error(`API Error on ${endpoint}:`, error);
    throw error;
  }
}

// ==========================================
// AUTHENTICATION LOGIC
// ==========================================
function checkAuth() {
  const authContainer = document.getElementById('auth-container');
  if (!STATE.token) {
    authContainer.classList.remove('hidden');
    document.getElementById('app-container').classList.add('pointer-events-none', 'blur-sm');
  } else {
    authContainer.classList.add('hidden');
    document.getElementById('app-container').classList.remove('pointer-events-none', 'blur-sm');
    
    // Set user UI details
    if (STATE.user) {
      document.getElementById('user-display-name').innerText = STATE.user.username;
      document.getElementById('user-avatar-initial').innerText = STATE.user.username.charAt(0).toUpperCase();
    }
    
    // Initialize services
    initWebSocket();
    pingDevice();
    loadDashboardMetrics();
  }
}

// ==========================================
// WEBSOCKET REAL-TIME BROADCASTS
// ==========================================
function initWebSocket() {
  if (STATE.ws) {
    STATE.ws.close();
  }

  const loc = window.location;
  let wsUri = loc.protocol === 'https:' ? 'wss:' : 'ws:';
  wsUri += `//${loc.host}/ws`;

  // Append JWT token for ws auth check
  wsUri += `?token=${STATE.token}`;

  STATE.ws = new WebSocket(wsUri);

  STATE.ws.onopen = () => {
    console.log('WS Connection established');
  };

  STATE.ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      handleWSMessage(msg);
    } catch (e) {
      console.error('Error parsing WS message:', e);
    }
  };

  STATE.ws.onclose = () => {
    console.log('WS Connection closed. Reconnecting in 3 seconds...');
    setTimeout(initWebSocket, 3000);
  };
}

function handleWSMessage(msg) {
  console.log('WS Message Received:', msg);
  const type = msg.type;
  const payload = msg.payload;

  switch (type) {
    case 'activity':
      appendActivityLog(payload);
      break;
    case 'new_file':
      showToast(`New shared file: ${payload.file_name}`, 'success');
      loadFilesList();
      loadDashboardMetrics();
      break;
    case 'file_delete':
      loadFilesList();
      loadDashboardMetrics();
      break;
    case 'new_snippet':
      showToast(`New code snippet: ${payload.title}`, 'success');
      loadSnippetsList();
      loadDashboardMetrics();
      break;
    case 'snippet_update':
      loadSnippetsList();
      // If we are currently editing this snippet, refresh the editor
      if (STATE.selectedSnippet && STATE.selectedSnippet.id === payload.id) {
        STATE.selectedSnippet = payload;
      }
      break;
    case 'snippet_delete':
      loadSnippetsList();
      if (STATE.selectedSnippet && STATE.selectedSnippet.id === payload) {
        unloadSnippetEditor();
      }
      break;
    case 'devices_list':
      renderDeviceList(payload);
      document.getElementById('stat-devices').innerText = payload.filter(d => d.status === 'online').length;
      break;
    case 'upload_progress':
      handleExternalUploadProgress(payload);
      break;
  }
}

// ==========================================
// DEVICE REGISTER & DISCOVERY
// ==========================================
async function pingDevice() {
  try {
    // Generate simple device name based on UserAgent
    const ua = navigator.userAgent;
    let deviceName = 'Browser Client';
    if (ua.match(/chrome|chromium|crios/i)) {
      deviceName = 'Chrome Browser';
    } else if (ua.match(/firefox|fxios/i)) {
      deviceName = 'Firefox Browser';
    } else if (ua.match(/safari/i)) {
      deviceName = 'Safari Browser';
    }
    if (ua.match(/mobile/i)) {
      deviceName += ' (Mobile)';
    }

    const device = await apiRequest('/api/devices/ping', 'POST', { device_name: deviceName });
    document.getElementById('user-display-ip').innerText = device.ip_address;
  } catch (err) {
    console.error('Failed to log device ping:', err);
  }
}

function renderDeviceList(devices) {
  const container = document.getElementById('device-list');
  if (!devices || devices.length === 0) {
    container.innerHTML = '<p class="text-sm text-slate-500 py-8 text-center">No active devices logged.</p>';
    return;
  }

  container.innerHTML = devices.map(d => {
    const isOnline = d.status === 'online';
    const statusColor = isOnline ? 'bg-green-500' : 'bg-slate-600';
    const activeTime = new Date(d.last_seen).toLocaleTimeString();
    
    return `
      <div class="glass-panel p-3.5 rounded-xl border border-slate-800/80 flex items-center justify-between">
        <div class="flex items-center gap-3 min-w-0">
          <div class="w-2.5 h-2.5 rounded-full ${statusColor} ${isOnline ? 'status-dot-pulse' : ''}"></div>
          <div class="min-w-0">
            <h4 class="text-xs font-semibold text-slate-300 truncate">${d.device_name}</h4>
            <p class="text-[10px] text-slate-500 font-mono mt-0.5">${d.ip_address}</p>
          </div>
        </div>
        <span class="text-[10px] text-slate-500 font-medium">Seen: ${activeTime}</span>
      </div>
    `;
  }).join('');
}

// ==========================================
// ACTIVITY LOG TIMELINE
// ==========================================
function appendActivityLog(log) {
  const container = document.getElementById('activity-log-timeline');
  if (container.querySelector('p')) {
    container.innerHTML = '';
  }

  const timeStr = new Date(log.created_at).toLocaleTimeString();
  const logItem = document.createElement('div');
  logItem.className = 'py-1 flex items-start gap-2 text-slate-400 hover:text-slate-200 transition-colors';
  
  let typeColor = 'text-blue-400';
  if (log.activity_type.startsWith('auth')) {
    typeColor = 'text-yellow-500';
  } else if (log.activity_type.startsWith('file')) {
    typeColor = 'text-green-400';
  } else if (log.activity_type.startsWith('snippet')) {
    typeColor = 'text-purple-400';
  }

  logItem.innerHTML = `
    <span class="text-slate-600 font-mono shrink-0">[${timeStr}]</span>
    <span class="${typeColor} font-bold shrink-0">&gt;</span>
    <span class="flex-1 break-all">${log.description}</span>
  `;

  container.insertBefore(logItem, container.firstChild);

  // Keep max 40 log lines
  while (container.children.length > 40) {
    container.removeChild(container.lastChild);
  }
}

// ==========================================
// DASHBOARD STATS
// ==========================================
async function loadDashboardMetrics() {
  try {
    const data = await apiRequest('/api/dashboard/metrics');
    STATE.localIP = data.local_ip;
    STATE.port = data.port;

    // Update Widgets
    document.getElementById('stat-files').innerText = data.total_files;
    document.getElementById('stat-snippets').innerText = data.total_snippets;
    document.getElementById('stat-devices').innerText = data.online_devices_count;
    document.getElementById('stat-storage').innerText = formatBytes(data.storage_usage_bytes);
    document.getElementById('stat-memory').innerText = data.memory_allocated_mb.toFixed(2) + ' MB';

    // Update IP badges
    document.getElementById('host-ip-badge').innerText = `${data.local_ip}:${data.port}`;

    // Render device discovery list
    renderDeviceList(data.devices);

    // Render activities
    const timeline = document.getElementById('activity-log-timeline');
    if (data.activities && data.activities.length > 0) {
      timeline.innerHTML = '';
      data.activities.forEach(log => {
        const timeStr = new Date(log.created_at).toLocaleTimeString();
        const logItem = document.createElement('div');
        logItem.className = 'py-1 flex items-start gap-2 text-slate-400 hover:text-slate-200 transition-colors';
        
        let typeColor = 'text-blue-400';
        if (log.activity_type.startsWith('auth')) {
          typeColor = 'text-yellow-500';
        } else if (log.activity_type.startsWith('file')) {
          typeColor = 'text-green-400';
        } else if (log.activity_type.startsWith('snippet')) {
          typeColor = 'text-purple-400';
        }

        logItem.innerHTML = `
          <span class="text-slate-600 font-mono shrink-0">[${timeStr}]</span>
          <span class="${typeColor} font-bold shrink-0">&gt;</span>
          <span class="flex-1 break-all">${log.description}</span>
        `;
        timeline.appendChild(logItem);
      });
    } else {
      timeline.innerHTML = '<p class="text-slate-600">Waiting for local activities...</p>';
    }
  } catch (err) {
    console.error('Failed to load dashboard metrics:', err);
  }
}

// ==========================================
// CODE VAULT: SNIPPETS
// ==========================================
async function loadSnippetsList() {
  const query = document.getElementById('snippet-search').value;
  const lang = document.getElementById('snippet-lang-filter').value;
  
  try {
    const list = await apiRequest(`/api/snippets?q=${encodeURIComponent(query)}&language=${lang}`);
    STATE.snippetsList = list || [];
    renderSnippetsList();
  } catch (err) {
    showToast('Failed to load snippets list', 'error');
  }
}

function renderSnippetsList() {
  const container = document.getElementById('snippets-list');
  if (STATE.snippetsList.length === 0) {
    container.innerHTML = '<p class="text-sm text-slate-500 text-center py-8">No snippets found.</p>';
    return;
  }

  container.innerHTML = STATE.snippetsList.map(s => {
    const isFav = s.is_favorite ? 'text-amber-500' : 'text-slate-600';
    return `
      <div onclick="selectSnippet('${s.id}')" class="glass-panel p-4 rounded-xl border border-slate-800/80 cursor-pointer glass-panel-hover flex flex-col justify-between gap-2 ${STATE.selectedSnippet && STATE.selectedSnippet.id === s.id ? 'border-blue-500/50 bg-blue-950/10 glow-blue' : ''}">
        <div class="flex items-start justify-between gap-2 min-w-0">
          <h4 class="text-sm font-bold text-slate-200 truncate flex-1">${s.title}</h4>
          <i class="fa-solid fa-star ${isFav} shrink-0"></i>
        </div>
        <div class="flex items-center justify-between text-[10px]">
          <span class="px-2 py-0.5 bg-slate-900 border border-slate-800 rounded font-semibold text-slate-400 uppercase tracking-wider">${s.language}</span>
          <span class="text-slate-500 font-mono">${new Date(s.created_at).toLocaleDateString()}</span>
        </div>
      </div>
    `;
  }).join('');
}

function selectSnippet(id) {
  const snippet = STATE.snippetsList.find(s => s.id === id);
  if (!snippet) return;
  STATE.selectedSnippet = snippet;
  renderSnippetsList();

  // Show Editor Panel
  document.getElementById('editor-placeholder').classList.add('hidden');
  const panel = document.getElementById('snippet-editor-panel');
  panel.classList.remove('hidden');

  // Load editor details
  document.getElementById('editor-title').innerText = snippet.title;
  document.getElementById('editor-lang-badge').innerText = snippet.language;
  document.getElementById('editor-tags').innerText = snippet.tags ? `tags: ${snippet.tags}` : 'tags: none';

  // Toggle favorite style
  const favBtn = document.getElementById('btn-editor-fav');
  if (snippet.is_favorite) {
    favBtn.className = 'p-2 rounded-lg bg-slate-900 border border-slate-800 text-amber-500 hover:text-amber-400';
  } else {
    favBtn.className = 'p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-500 hover:text-amber-400';
  }

  // Toggle markdown preview button
  const previewBtn = document.getElementById('btn-editor-preview');
  if (snippet.language.toLowerCase() === 'markdown') {
    previewBtn.classList.remove('hidden');
  } else {
    previewBtn.classList.add('hidden');
  }

  // Reset editor view (default to Monaco code editor)
  document.getElementById('markdown-preview-container').classList.add('hidden');
  document.getElementById('monaco-editor-container').classList.remove('hidden');
  previewBtn.innerHTML = '<i class="fa-solid fa-book-open"></i> Preview';

  // Load Monaco editor content
  if (STATE.editor) {
    STATE.editor.setValue(snippet.content);
    let monacoLang = snippet.language;
    // Map database languages to Monaco editor models
    if (monacoLang === 'go') monacoLang = 'go';
    else if (monacoLang === 'javascript') monacoLang = 'javascript';
    else if (monacoLang === 'python') monacoLang = 'python';
    monaco.editor.setModelLanguage(STATE.editor.getModel(), monacoLang);
  }
}

function unloadSnippetEditor() {
  STATE.selectedSnippet = null;
  document.getElementById('snippet-editor-panel').classList.add('hidden');
  document.getElementById('editor-placeholder').classList.remove('hidden');
  renderSnippetsList();
}

// ==========================================
// MONACO EDITOR DYNAMIC LOADING
// ==========================================
function initMonacoEditor() {
  require.config({ paths: { vs: 'https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.45.0/min/vs' } });

  require(['vs/editor/editor.main'], function () {
    // Define Dark Developer Theme
    monaco.editor.defineTheme('devsyncDark', {
      base: 'vs-dark',
      inherit: true,
      rules: [
        { token: 'comment', foreground: '6272a4', fontStyle: 'italic' },
        { token: 'keyword', foreground: 'ff79c6' },
        { token: 'identifier', foreground: 'f8f8f2' },
        { token: 'string', foreground: 'f1fa8c' },
        { token: 'number', foreground: 'bd93f9' },
      ],
      colors: {
        'editor.background': '#0b0f19',
        'editor.foreground': '#f8f8f2',
        'editor.lineHighlightBackground': '#1e293b50',
        'editorLineNumber.foreground': '#475569',
        'editorLineNumber.activeForeground': '#3b82f6',
      }
    });

    STATE.editor = monaco.editor.create(document.getElementById('monaco-editor-container'), {
      value: "",
      language: "python",
      theme: "devsyncDark",
      fontSize: 13,
      fontFamily: "'Fira Code', monospace",
      automaticLayout: true,
      minimap: { enabled: false },
      lineNumbers: "on",
      roundedSelection: true,
      scrollBeyondLastLine: false,
      readOnly: false,
    });
  });
}

// ==========================================
// FILE SHARING MODULE: CHUNK UPLOAD
// ==========================================
const CHUNK_SIZE = 1024 * 1024; // 1MB Chunk size

async function uploadFileInChunks(file, relativePath = '') {
  const uploadUUID = crypto.randomUUID();
  const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
  
  const progressPanel = document.getElementById('upload-progress-panel');
  const progressBar = document.getElementById('upload-progress-bar');
  const percentText = document.getElementById('upload-percentage');
  const sizeText = document.getElementById('upload-size');
  const speedText = document.getElementById('upload-speed');
  
  progressPanel.classList.remove('hidden');
  
  const startTime = Date.now();
  let uploadedBytes = 0;

  for (let index = 0; index < totalChunks; index++) {
    const start = index * CHUNK_SIZE;
    const end = Math.min(file.size, start + CHUNK_SIZE);
    const blob = file.slice(start, end);

    const formData = new FormData();
    formData.append('chunk', blob, file.name);
    formData.append('chunk_index', index);
    formData.append('total_chunks', totalChunks);
    formData.append('upload_uuid', uploadUUID);

    try {
      await apiRequest('/api/files/upload-chunk', 'POST', formData, true);
      uploadedBytes += (end - start);
      
      // Calculate statistics
      const elapsedSeconds = (Date.now() - startTime) / 1000;
      const speed = elapsedSeconds > 0 ? uploadedBytes / elapsedSeconds : 0;
      const progressPercent = Math.round((uploadedBytes / file.size) * 100);
      
      progressBar.style.width = `${progressPercent}%`;
      percentText.innerText = `${progressPercent}%`;
      sizeText.innerText = `${formatBytes(uploadedBytes)} of ${formatBytes(file.size)}`;
      speedText.innerText = `Speed: ${(speed / 1024 / 1024).toFixed(2)} MB/s`;

    } catch (err) {
      showToast(`Upload failed for chunk ${index}`, 'error');
      progressPanel.classList.add('hidden');
      return;
    }
  }

  // Merge Chunks API
  try {
    percentText.innerText = 'Assembling...';
    const payload = {
      upload_uuid: uploadUUID,
      file_name: file.name,
      total_chunks: totalChunks,
      file_size: file.size,
      sender_device: STATE.user ? STATE.user.username : 'WebClient',
      relative_path: relativePath,
    };
    
    await apiRequest('/api/files/merge-chunks', 'POST', payload);
    showToast(`File "${file.name}" uploaded completely!`, 'success');
    
    // Hide panel & refresh lists
    setTimeout(() => {
      progressPanel.classList.add('hidden');
      progressBar.style.width = '0%';
      percentText.innerText = '0%';
    }, 1000);
    
    loadFilesList();
    loadDashboardMetrics();
  } catch (err) {
    showToast(`Failed to merge uploaded chunks: ${err.message}`, 'error');
    progressPanel.classList.add('hidden');
  }
}

// Websocket-driven remote chunk progress update (handles progress broadcasted by other devices)
function handleExternalUploadProgress(data) {
  // Can be used to show a transfer indicator in real-time widgets.
}

async function loadFilesList() {
  const query = document.getElementById('file-search').value;
  try {
    const list = await apiRequest(`/api/files?q=${encodeURIComponent(query)}`);
    STATE.filesList = list || [];
    renderFilesList();
    buildFolderTree();
  } catch (err) {
    showToast('Failed to load shared files', 'error');
  }
}

function renderFilesList() {
  const container = document.getElementById('files-list');
  if (STATE.filesList.length === 0) {
    container.innerHTML = '<p class="text-sm text-slate-500 text-center py-12 w-full col-span-2">No files shared yet. Drag and drop to share!</p>';
    return;
  }

  container.innerHTML = STATE.filesList.map(f => {
    let icon = '<i class="fa-solid fa-file text-slate-400"></i>';
    const ext = f.file_type.toLowerCase();
    if (ext === '.zip' || ext === '.rar' || ext === '.tar') {
      icon = '<i class="fa-solid fa-file-zipper text-yellow-500"></i>';
    } else if (ext === '.png' || ext === '.jpg' || ext === '.svg' || ext === '.jpeg') {
      icon = '<i class="fa-solid fa-file-image text-emerald-400"></i>';
    } else if (ext === '.mp4' || ext === '.mkv' || ext === '.avi') {
      icon = '<i class="fa-solid fa-file-video text-purple-400"></i>';
    } else if (ext === '.pdf') {
      icon = '<i class="fa-solid fa-file-pdf text-red-500"></i>';
    } else if (ext === '.go' || ext === '.py' || ext === '.js' || ext === '.html' || ext === '.css' || ext === '.java') {
      icon = '<i class="fa-solid fa-file-code text-blue-400"></i>';
    }

    return `
      <div class="glass-panel p-5 rounded-2xl border border-slate-800/80 glass-panel-hover flex flex-col justify-between gap-4">
        <div class="flex items-start gap-4 min-w-0">
          <div class="w-11 h-11 bg-slate-900 border border-slate-800 rounded-xl flex items-center justify-center text-xl shrink-0">
            ${icon}
          </div>
          <div class="min-w-0 flex-1">
            <h4 class="text-sm font-bold text-slate-200 truncate" title="${f.file_name}">${f.file_name}</h4>
            <p class="text-[10px] text-slate-500 font-semibold font-mono mt-0.5">${formatBytes(f.file_size)}</p>
          </div>
        </div>
        
        <div class="flex items-center justify-between text-[10px] text-slate-500 font-semibold border-t border-slate-800/60 pt-3">
          <span>By: ${f.sender_device}</span>
          <span>Downloads: <b class="text-slate-400" id="dl-cnt-${f.id}">${f.download_count}</b></span>
        </div>

        <div class="flex gap-2">
          <button onclick="downloadFile('${f.id}', '${f.file_name}')" class="flex-1 py-2 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-xs font-semibold flex items-center justify-center gap-1.5 text-slate-300">
            <i class="fa-solid fa-download"></i> Download
          </button>
          <button onclick="openPreview('${f.id}')" class="px-3 py-2 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-xs font-semibold flex items-center justify-center gap-1.5 text-slate-300">
            <i class="fa-solid fa-eye"></i> Preview
          </button>
          <button onclick="deleteFile('${f.id}')" class="px-2 py-2 bg-red-950/15 border border-red-900/20 hover:border-red-900/50 hover:bg-red-900/10 text-red-500 hover:text-red-400 rounded-lg text-xs">
            <i class="fa-solid fa-trash"></i>
          </button>
        </div>
      </div>
    `;
  }).join('');
}

async function deleteFile(id) {
  if (!confirm('Are you sure you want to delete this file from local storage?')) return;
  try {
    await apiRequest(`/api/files/${id}`, 'DELETE');
    showToast('File deleted successfully', 'success');
  } catch (err) {
    showToast('Failed to delete file', 'error');
  }
}

// ==========================================
// CLI & QR CONNECTIONS
// ==========================================
function updateCLIConnection() {
  // Prefer the browser's current host (useful when user accessed via LAN IP).
  // Fall back to server-reported local IP when hostname is localhost/127.0.0.1.
  const loc = window.location;
  let host = loc.hostname;
  if (!host || host === 'localhost' || host === '127.0.0.1') {
    if (STATE.localIP && STATE.localIP !== '127.0.0.1') {
      host = STATE.localIP;
    }
  }
  const hostURL = `${loc.protocol}//${host}${loc.port ? ':' + loc.port : ''}`;

  // Set link and QR string
  const urlLink = document.getElementById('cli-connection-url');
  urlLink.innerText = hostURL;
  urlLink.href = hostURL;

  // Set cURL snippets
  document.getElementById('curl-cmd-1').innerText = `curl -F "file=@project.zip" ${hostURL}/api/files/upload`;
  document.getElementById('curl-cmd-2').innerText = `curl -F "file=@notes.pdf" -F "sender_device=MacBookCLI" ${hostURL}/api/files/upload`;

  // Draw QR Code
  const qrContainer = document.getElementById('qrcode-container');
  qrContainer.innerHTML = '';
  new QRCode(qrContainer, {
    text: hostURL,
    width: 160,
    height: 160,
    colorDark: "#020617",
    colorLight: "#ffffff",
    correctLevel: QRCode.CorrectLevel.H
  });
}

// ==========================================
// TAB ROUTING & INITIALIZATIONS
// ==========================================
document.querySelectorAll('.nav-item').forEach(item => {
  item.addEventListener('click', (e) => {
    e.preventDefault();
    const tabName = item.getAttribute('data-tab');
    switchTab(tabName);
  });
});

function switchTab(tabName, opts = { push: true }) {
  STATE.activeTab = tabName;

  // Active styles in Sidebar
  document.querySelectorAll('.nav-item').forEach(i => {
    if (i.getAttribute('data-tab') === tabName) {
      i.className = 'nav-item flex items-center gap-3 px-4 py-3.5 rounded-xl text-sm font-semibold bg-gradient-to-r from-blue-600/10 to-purple-600/10 border-l-2 border-blue-500 text-blue-400 shadow-sm';
    } else {
      i.className = 'nav-item flex items-center gap-3 px-4 py-3.5 rounded-xl text-sm font-medium text-slate-400 hover:text-white hover:bg-slate-900/60 transition-all duration-200';
    }
  });

  // Hide all sections, show active
  document.querySelectorAll('.tab-section').forEach(sec => {
    sec.classList.add('hidden');
  });
  
  const activeSec = document.getElementById(`tab-${tabName}`);
  if (activeSec) activeSec.classList.remove('hidden');

  // Set header title
  const titles = {
    dashboard: 'Dashboard Monitoring',
    snippets: 'Developer Code Vault',
    files: 'File Sharing Vault',
    cli: 'CLI & Connection Portal',
    login: 'Sign In',
    register: 'Create Account'
  };
  const titleEl = document.getElementById('current-section-title');
  if (titleEl) titleEl.innerText = titles[tabName] || '';

  // Refresh tab data
  if (tabName === 'dashboard') {
    loadDashboardMetrics();
  } else if (tabName === 'snippets') {
    loadSnippetsList();
    unloadSnippetEditor();
  } else if (tabName === 'files') {
    loadFilesList();
  } else if (tabName === 'cli') {
    updateCLIConnection();
  }

  // Update browser URL using history API so routes are shareable/bookmarkable
  if (opts.push) {
    const pathMap = {
      'dashboard': '/dashboard',
      'snippets': '/snippets',
      'files': '/files',
      'cli': '/cli',
      'login': '/login',
      'register': '/register'
    };
    const newPath = pathMap[tabName] || '/';
    try { history.pushState({ tab: tabName }, '', newPath); } catch (e) { /* ignore */ }
  }
}

// Map pathname -> tab name
function pathToTab(pathname) {
  if (!pathname) return 'dashboard';
  const p = pathname.replace(/\/$/, '');
  switch (p) {
    case '':
    case '/':
    case '/dashboard':
      return 'dashboard';
    case '/snippets':
      return 'snippets';
    case '/files':
      return 'files';
    case '/cli':
      return 'cli';
    case '/login':
      return 'login';
    case '/register':
      return 'register';
    default:
      return 'dashboard';
  }
}

// Handle back/forward navigation
window.addEventListener('popstate', () => {
  const tab = pathToTab(window.location.pathname);
  switchTab(tab, { push: false });
});

// On initial load, pick tab from URL
document.addEventListener('DOMContentLoaded', () => {
  const initial = pathToTab(window.location.pathname);
  switchTab(initial, { push: false });
});

// ==========================================
// EVENT LISTENERS & INITIAL BOOT
// ==========================================

// Drag & drop file upload
const dropZone = document.getElementById('drop-zone');
dropZone.addEventListener('dragover', (e) => {
  e.preventDefault();
  dropZone.classList.add('border-blue-500', 'bg-blue-950/10');
});
dropZone.addEventListener('dragleave', () => {
  dropZone.classList.remove('border-blue-500', 'bg-blue-950/10');
});
dropZone.addEventListener('drop', (e) => {
  e.preventDefault();
  dropZone.classList.remove('border-blue-500', 'bg-blue-950/10');
  const files = e.dataTransfer.files;
  if (files.length > 0) {
    for (let f of files) {
      uploadFileInChunks(f);
    }
  }
});
dropZone.addEventListener('click', () => {
  document.getElementById('file-input').click();
});
document.getElementById('file-input').addEventListener('change', (e) => {
  const files = e.target.files;
  if (files.length > 0) {
    for (let f of files) {
      uploadFileInChunks(f);
    }
  }
});
document.getElementById('folder-input').addEventListener('change', (e) => {
  const files = e.target.files;
  if (files.length > 0) {
    for (let f of files) {
      uploadFileInChunks(f, f.webkitRelativePath || '');
    }
  }
});

// File Type Filter Buttons
document.querySelectorAll('.file-filter-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.file-filter-btn').forEach(b => {
      b.className = 'file-filter-btn w-full px-4 py-2.5 rounded-xl text-sm text-left hover:bg-slate-900 border border-slate-800/40 text-slate-400 hover:text-white flex items-center justify-between';
    });
    btn.className = 'file-filter-btn w-full px-4 py-2.5 rounded-xl text-sm text-left bg-slate-900 border border-slate-800 text-blue-400 font-semibold flex items-center justify-between';
    
    const extensionsStr = btn.getAttribute('data-type');
    if (extensionsStr === "") {
      loadFilesList();
      return;
    }

    const extensions = extensionsStr.split(',');
    // filter state.filesList local copy
    const filtered = STATE.filesList.filter(f => {
      const ext = f.file_type.toLowerCase();
      return extensions.includes(ext);
    });
    const container = document.getElementById('files-list');
    if (filtered.length === 0) {
      container.innerHTML = '<p class="text-sm text-slate-500 text-center py-12 w-full col-span-2">No matching files found.</p>';
      return;
    }
    
    // Render filtered list
    container.innerHTML = filtered.map(f => {
      let icon = '<i class="fa-solid fa-file text-slate-400"></i>';
      const ext = f.file_type.toLowerCase();
      if (ext === '.zip' || ext === '.rar' || ext === '.tar') {
        icon = '<i class="fa-solid fa-file-zipper text-yellow-500"></i>';
      } else if (ext === '.png' || ext === '.jpg' || ext === '.svg' || ext === '.jpeg') {
        icon = '<i class="fa-solid fa-file-image text-emerald-400"></i>';
      } else if (ext === '.mp4' || ext === '.mkv' || ext === '.avi') {
        icon = '<i class="fa-solid fa-file-video text-purple-400"></i>';
      } else if (ext === '.pdf') {
        icon = '<i class="fa-solid fa-file-pdf text-red-500"></i>';
      }

      return `
        <div class="glass-panel p-5 rounded-2xl border border-slate-800/80 glass-panel-hover flex flex-col justify-between gap-4">
          <div class="flex items-start gap-4 min-w-0">
            <div class="w-11 h-11 bg-slate-900 border border-slate-800 rounded-xl flex items-center justify-center text-xl shrink-0">
              ${icon}
            </div>
            <div class="min-w-0 flex-1">
              <h4 class="text-sm font-bold text-slate-200 truncate">${f.file_name}</h4>
              <p class="text-[10px] text-slate-500 font-semibold font-mono mt-0.5">${formatBytes(f.file_size)}</p>
            </div>
          </div>
          <div class="flex items-center justify-between text-[10px] text-slate-500 font-semibold border-t border-slate-800/60 pt-3">
            <span>By: ${f.sender_device}</span>
            <span>Downloads: <b class="text-slate-400">${f.download_count}</b></span>
          </div>
          <div class="flex gap-2.5">
            <a href="/api/files/download/${f.id}" target="_blank" class="flex-1 py-2 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-lg text-xs font-semibold flex items-center justify-center gap-1.5 text-slate-300">
              <i class="fa-solid fa-download"></i> Download
            </a>
            <button onclick="deleteFile('${f.id}')" class="px-3 py-2 bg-red-950/15 border border-red-900/20 hover:border-red-900/50 hover:bg-red-900/10 text-red-500 hover:text-red-400 rounded-lg text-xs">
              <i class="fa-solid fa-trash"></i>
            </button>
          </div>
        </div>
      `;
    }).join('');
  });
});

// File search listener
document.getElementById('file-search').addEventListener('input', loadFilesList);

// Snippet Search & filters
document.getElementById('snippet-search').addEventListener('input', loadSnippetsList);
document.getElementById('snippet-lang-filter').addEventListener('change', loadSnippetsList);
document.getElementById('snippet-fav-filter').addEventListener('click', () => {
  const btn = document.getElementById('snippet-fav-filter');
  if (btn.classList.contains('bg-slate-900')) {
    btn.className = 'px-3 py-2 rounded-xl text-xs border border-amber-500/20 bg-amber-500/10 text-amber-400 flex items-center justify-center gap-2 glow-purple';
    // Filter local list to show favorites only
    const filtered = STATE.snippetsList.filter(s => s.is_favorite);
    STATE.snippetsList = filtered;
    renderSnippetsList();
  } else {
    btn.className = 'px-3 py-2 rounded-xl text-xs border border-slate-800 bg-slate-900/40 hover:bg-slate-900 text-slate-400 flex items-center justify-center gap-2';
    loadSnippetsList();
  }
});

// Monaco Editor actions: copy, download, save, delete
document.getElementById('btn-editor-copy').addEventListener('click', () => {
  if (STATE.editor) {
    const code = STATE.editor.getValue();
    navigator.clipboard.writeText(code).then(() => {
      showToast('Code copied!', 'success');
    });
  }
});

document.getElementById('btn-editor-download').addEventListener('click', () => {
  if (!STATE.selectedSnippet || !STATE.editor) return;
  const code = STATE.editor.getValue();
  const blob = new Blob([code], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = STATE.selectedSnippet.title.toLowerCase().replace(/[^a-z0-9]/g, '_') + '.' + STATE.selectedSnippet.language;
  a.click();
  URL.revokeObjectURL(url);
});

document.getElementById('btn-editor-fav').addEventListener('click', async () => {
  if (!STATE.selectedSnippet) return;
  const targetFav = !STATE.selectedSnippet.is_favorite;
  try {
    const updated = await apiRequest(`/api/snippets/${STATE.selectedSnippet.id}/favorite`, 'POST', { is_favorite: targetFav });
    STATE.selectedSnippet = updated;
    selectSnippet(updated.id);
    loadSnippetsList();
    showToast(targetFav ? 'Added to favorites' : 'Removed from favorites', 'success');
  } catch (err) {
    showToast('Failed to toggle favorite', 'error');
  }
});

document.getElementById('btn-editor-save').addEventListener('click', async () => {
  if (!STATE.selectedSnippet || !STATE.editor) return;
  const content = STATE.editor.getValue();
  try {
    const payload = {
      title: STATE.selectedSnippet.title,
      language: STATE.selectedSnippet.language,
      tags: STATE.selectedSnippet.tags,
      is_favorite: STATE.selectedSnippet.is_favorite,
      content: content,
    };
    await apiRequest(`/api/snippets/${STATE.selectedSnippet.id}`, 'PUT', payload);
    showToast('Snippet code updated!', 'success');
  } catch (err) {
    showToast('Failed to save code updates', 'error');
  }
});

document.getElementById('btn-editor-delete').addEventListener('click', () => {
  if (STATE.selectedSnippet) {
    deleteSnippet(STATE.selectedSnippet.id);
  }
});

async function deleteSnippet(id) {
  if (!confirm('Are you sure you want to delete this snippet?')) return;
  try {
    await apiRequest(`/api/snippets/${id}`, 'DELETE');
    showToast('Snippet deleted', 'success');
  } catch (err) {
    showToast('Failed to delete snippet', 'error');
  }
}

// Snippet Modal actions
const snippetModal = document.getElementById('snippet-modal');
document.getElementById('show-create-snippet-modal').addEventListener('click', () => {
  document.getElementById('modal-snippet-id').value = '';
  document.getElementById('modal-snippet-title').value = '';
  document.getElementById('modal-snippet-tags').value = '';
  document.getElementById('modal-snippet-content').value = '';
  document.getElementById('modal-snippet-fav').checked = false;
  document.getElementById('snippet-modal-title').innerText = 'Create Code Snippet';
  snippetModal.classList.remove('hidden');
});

document.getElementById('close-snippet-modal-btn').addEventListener('click', () => {
  snippetModal.classList.add('hidden');
});

document.getElementById('snippet-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  
  const title = document.getElementById('modal-snippet-title').value;
  const language = document.getElementById('modal-snippet-lang').value;
  const tags = document.getElementById('modal-snippet-tags').value;
  const content = document.getElementById('modal-snippet-content').value;
  const isFavorite = document.getElementById('modal-snippet-fav').checked;

  const payload = { title, language, tags, content, is_favorite: isFavorite };

  try {
    await apiRequest('/api/snippets', 'POST', payload);
    showToast('Snippet created!', 'success');
    snippetModal.classList.add('hidden');
    loadSnippetsList();
  } catch (err) {
    showToast(`Failed to create snippet: ${err.message}`, 'error');
  }
});

// Auth form submissions
document.getElementById('login-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const username = document.getElementById('login-username').value;
  const password = document.getElementById('login-password').value;

  try {
    const res = await apiRequest('/api/auth/login', 'POST', { username, password });
    STATE.token = res.token;
    STATE.user = res.user;
    localStorage.setItem('devsync_token', res.token);
    localStorage.setItem('devsync_user', JSON.stringify(res.user));
    
    showToast('Authenticated successfully!', 'success');
    checkAuth();
  } catch (err) {
    showToast(err.message, 'error');
  }
});

document.getElementById('register-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const username = document.getElementById('reg-username').value;
  const email = document.getElementById('reg-email').value;
  const password = document.getElementById('reg-password').value;

  try {
    const res = await apiRequest('/api/auth/register', 'POST', { username, email, password });
    STATE.token = res.token;
    STATE.user = res.user;
    localStorage.setItem('devsync_token', res.token);
    localStorage.setItem('devsync_user', JSON.stringify(res.user));
    
    showToast('Profile created successfully!', 'success');
    checkAuth();
  } catch (err) {
    showToast(err.message, 'error');
  }
});

document.getElementById('show-register-btn').addEventListener('click', (e) => {
  e.preventDefault();
  document.getElementById('login-form').classList.add('hidden');
  document.getElementById('register-form').classList.remove('hidden');
});

document.getElementById('show-login-btn').addEventListener('click', (e) => {
  e.preventDefault();
  document.getElementById('register-form').classList.add('hidden');
  document.getElementById('login-form').classList.remove('hidden');
});

document.getElementById('logout-btn').addEventListener('click', () => {
  if (confirm('Are you sure you want to sign out?')) {
    localStorage.removeItem('devsync_token');
    localStorage.removeItem('devsync_user');
    STATE.token = '';
    STATE.user = null;
    if (STATE.ws) {
      STATE.ws.close();
    }
    showToast('Logged out');
    checkAuth();
  }
});

// Boot App
window.addEventListener('DOMContentLoaded', () => {
  checkAuth();
  initMonacoEditor();
  switchTab('dashboard');

  // Markdown Preview Toggle Listener
  document.getElementById('btn-editor-preview').addEventListener('click', () => {
    const previewContainer = document.getElementById('markdown-preview-container');
    const editorContainer = document.getElementById('monaco-editor-container');
    const btn = document.getElementById('btn-editor-preview');

    if (previewContainer.classList.contains('hidden')) {
      previewContainer.classList.remove('hidden');
      editorContainer.classList.add('hidden');
      
      const markdownText = STATE.editor ? STATE.editor.getValue() : '';
      previewContainer.innerHTML = marked.parse(markdownText);
      btn.innerHTML = '<i class="fa-solid fa-code"></i> Editor';
      btn.title = "Switch to Editor";
    } else {
      previewContainer.classList.add('hidden');
      editorContainer.classList.remove('hidden');
      btn.innerHTML = '<i class="fa-solid fa-book-open"></i> Preview';
      btn.title = "Switch to Live Preview";
    }
  });

  // Preview Modal Listeners
  document.getElementById('close-preview-btn').addEventListener('click', closePreview);
  document.getElementById('preview-modal').addEventListener('click', (e) => {
    if (e.target.id === 'preview-modal') closePreview();
  });
});

// ==========================================
// FILE PREVIEW MODAL LOGIC
// ==========================================
async function openPreview(id) {
  const f = STATE.filesList.find(item => item.id === id);
  if (!f) return;

  const modal = document.getElementById('preview-modal');
  document.getElementById('preview-filename').innerText = f.file_name;
  document.getElementById('preview-filesize').innerText = formatBytes(f.file_size);
  document.getElementById('preview-meta').innerText = `Uploaded by: ${f.sender_device}`;
  document.getElementById('preview-time').innerText = `Time: ${new Date(f.uploaded_at).toLocaleString()}`;
  
  const dlBtn = document.getElementById('preview-download-btn');
  dlBtn.onclick = (e) => {
    e.preventDefault();
    downloadFile(f.id, f.file_name);
  };

  const container = document.getElementById('preview-content-container');
  container.innerHTML = '<p class="text-slate-500 text-xs">Loading preview...</p>';
  modal.classList.remove('hidden');

  const ext = f.file_type.toLowerCase();
  
  if (['.png', '.jpg', '.jpeg', '.gif', '.svg'].includes(ext)) {
    container.innerHTML = `<img src="/api/files/download/${f.id}" class="max-h-[50vh] max-w-full rounded-lg object-contain shadow-md">`;
  } else if (['.mp4', '.mkv', '.avi', '.mov', '.webm'].includes(ext)) {
    container.innerHTML = `<video src="/api/files/download/${f.id}" controls class="max-h-[50vh] max-w-full rounded-lg shadow-md"></video>`;
  } else if (['.mp3', '.wav', '.ogg'].includes(ext)) {
    container.innerHTML = `<audio src="/api/files/download/${f.id}" controls class="w-full max-w-md"></audio>`;
  } else if (ext === '.pdf') {
    container.innerHTML = `<iframe src="/api/files/download/${f.id}" class="w-full h-[50vh] rounded-lg border border-slate-800"></iframe>`;
  } else if (['.txt', '.md', '.go', '.py', '.js', '.css', '.html', '.json', '.sql', '.yaml', '.sh'].includes(ext)) {
    try {
      const response = await fetch(`/api/files/download/${f.id}`, {
        headers: STATE.token ? { 'Authorization': `Bearer ${STATE.token}` } : {}
      });
      if (!response.ok) throw new Error();
      const text = await response.text();
      
      if (ext === '.md') {
        container.innerHTML = `<div class="w-full text-left p-4 overflow-auto prose prose-invert text-slate-300 max-h-[50vh]">${marked.parse(text)}</div>`;
      } else {
        const escapedText = text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
        container.innerHTML = `<pre class="w-full text-left p-4 overflow-auto font-mono text-[11px] text-slate-300 select-text max-h-[50vh] bg-slate-950/60 rounded-lg whitespace-pre-wrap">${escapedText}</pre>`;
      }
    } catch (err) {
      container.innerHTML = '<p class="text-red-400 text-xs">Failed to load text preview content.</p>';
    }
  } else {
    container.innerHTML = `
      <div class="text-center p-8">
        <i class="fa-solid fa-file-circle-question text-slate-600 text-4xl mb-3"></i>
        <p class="text-xs text-slate-400">Preview not supported for this file type.</p>
      </div>
    `;
  }
}

function closePreview() {
  const modal = document.getElementById('preview-modal');
  modal.classList.add('hidden');
  const container = document.getElementById('preview-content-container');
  container.innerHTML = '';
}

// ==========================================
// JS CHUNK STREAM DOWNLOAD LOGIC
// ==========================================
async function downloadFile(id, filename) {
  const container = document.getElementById('toast-container');
  const widget = document.createElement('div');
  widget.className = `glass-panel px-5 py-4 rounded-xl border border-blue-500/20 glow-blue flex flex-col gap-2 text-sm shadow-xl z-50 w-80`;
  widget.id = `dl-widget-${id}`;
  widget.innerHTML = `
    <div class="flex justify-between items-center">
      <span class="font-bold text-slate-200 truncate flex-1 pr-2 text-xs">${filename}</span>
      <span class="text-xs font-mono text-blue-400 font-bold" id="dl-percent-${id}">0%</span>
    </div>
    <div class="w-full bg-slate-900 rounded-full h-1.5 overflow-hidden">
      <div class="bg-gradient-to-r from-blue-500 to-purple-500 h-1.5 rounded-full" id="dl-bar-${id}" style="width: 0%"></div>
    </div>
    <div class="flex justify-between text-[10px] text-slate-500 font-semibold">
      <span id="dl-size-${id}">0 B</span>
      <span id="dl-speed-${id}">0 MB/s</span>
    </div>
  `;
  container.appendChild(widget);

  try {
    const response = await fetch(`/api/files/download/${id}`, {
      headers: STATE.token ? { 'Authorization': `Bearer ${STATE.token}` } : {}
    });
    if (!response.ok) throw new Error('Download failed');
    
    const reader = response.body.getReader();
    const contentLength = +response.headers.get('Content-Length');
    
    let receivedBytes = 0;
    const chunks = [];
    const startTime = Date.now();

    while(true) {
      const {done, value} = await reader.read();
      if (done) break;
      
      chunks.push(value);
      receivedBytes += value.length;

      const elapsedSeconds = (Date.now() - startTime) / 1000;
      const speed = elapsedSeconds > 0 ? receivedBytes / elapsedSeconds : 0;
      const percent = contentLength ? Math.round((receivedBytes / contentLength) * 100) : 0;

      document.getElementById(`dl-bar-${id}`).style.width = `${percent}%`;
      document.getElementById(`dl-percent-${id}`).innerText = `${percent}%`;
      document.getElementById(`dl-size-${id}`).innerText = `${formatBytes(receivedBytes)} ${contentLength ? 'of ' + formatBytes(contentLength) : ''}`;
      document.getElementById(`dl-speed-${id}`).innerText = `Speed: ${(speed / 1024 / 1024).toFixed(2)} MB/s`;
    }

    const blob = new Blob(chunks);
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);

    showToast(`Download finished: ${filename}`, 'success');
    setTimeout(() => widget.remove(), 1000);

    const cntEl = document.getElementById(`dl-cnt-${id}`);
    if (cntEl) cntEl.innerText = parseInt(cntEl.innerText) + 1;

  } catch (err) {
    console.error(err);
    showToast(`Download failed: ${filename}`, 'error');
    widget.remove();
  }
}

// ==========================================
// INTERACTIVE FOLDER TREE VIEW LOGIC
// ==========================================
function buildFolderTree() {
  const container = document.getElementById('folder-tree-container');
  if (!STATE.filesList || STATE.filesList.length === 0) {
    container.innerHTML = '<p class="text-slate-500 text-center py-12">No folders uploaded yet.</p>';
    return;
  }

  const root = { _folders: {}, _files: [] };
  let hasFolders = false;

  STATE.filesList.forEach(f => {
    if (!f.relative_path || !f.relative_path.includes('/')) {
      return;
    }
    hasFolders = true;
    const parts = f.relative_path.split('/');
    let current = root;
    
    parts.forEach((part, i) => {
      if (i === parts.length - 1) {
        current._files.push(f);
      } else {
        if (!current._folders[part]) {
          current._folders[part] = { _folders: {}, _files: [] };
        }
        current = current._folders[part];
      }
    });
  });

  if (!hasFolders) {
    container.innerHTML = '<p class="text-slate-500 text-center py-12">No folders uploaded yet.</p>';
    return;
  }

  function renderNode(name, node, currentPath = '') {
    const fullPath = currentPath ? `${currentPath}/${name}` : name;
    
    let subFoldersHtml = Object.keys(node._folders).map(subName => 
      renderNode(subName, node._folders[subName], fullPath)
    ).join('');

    let filesHtml = node._files.map(f => `
      <div onclick="openPreview('${f.id}')" class="pl-4 py-1.5 flex items-center gap-2 hover:text-blue-400 cursor-pointer font-mono text-[11px] truncate text-slate-400">
        <i class="fa-solid fa-file-lines text-slate-500"></i>
        <span>${f.file_name}</span>
      </div>
    `).join('');

    return `
      <details class="pl-2 mt-1" open>
        <summary class="flex items-center gap-2 hover:text-white cursor-pointer font-semibold py-1 text-slate-300">
          <i class="fa-solid fa-folder text-yellow-500 text-xs"></i>
          <span onclick="filterByFolderPath('${fullPath}')">${name}</span>
        </summary>
        <div class="pl-2 border-l border-slate-800/80">
          ${subFoldersHtml}
          ${filesHtml}
        </div>
      </details>
    `;
  }

  let html = Object.keys(root._folders).map(folderName => 
    renderNode(folderName, root._folders[folderName])
  ).join('');

  container.innerHTML = html;
}

window.filterByFolderPath = function(folderPath) {
  event.stopPropagation();
  event.preventDefault();
  const filtered = STATE.filesList.filter(f => f.relative_path && f.relative_path.startsWith(folderPath));
  renderFilteredFiles(filtered);
}

