'use strict';
/**
 * 配对路由（参考实现）
 *
 * 端口分工：
 *   数据端口(0.0.0.0)  : POST /_pair/request, GET /_pair/status   ← 对方来访问
 *   控制端口(127.0.0.1): GET /pair/requests, POST /pair/approve   ← 只有本机 UI 用
 *
 * 控制端口必须做 loopback 校验 + CORS 白名单（管理页可能是 WebView 加载的）。
 */

const http = require('http');
const { PairStore } = require('./pair-store');

/* ---------------- 控制端口基线 ---------------- */

const LOOPBACK = new Set(['127.0.0.1', '::1', '::ffff:127.0.0.1']);
// 管理页与数据端口同源（http://127.0.0.1:<dataPort>），只放本机来源
const LOCAL_ORIGIN = /^https?:\/\/(127\.0\.0\.1|localhost)(:\d+)?$/;

function controlGuard(req, res, next) {
  if (!LOOPBACK.has(req.socket.remoteAddress)) {
    res.status(403).json({ error: 'forbidden', reason: 'control port is loopback-only' });
    return;
  }
  next();
}

function localCors(req, res, next) {
  const origin = req.headers.origin || '';
  if (LOCAL_ORIGIN.test(origin)) {
    res.setHeader('Access-Control-Allow-Origin', origin);
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
    res.setHeader('Access-Control-Allow-Methods', 'GET,POST,OPTIONS');
    res.setHeader('Vary', 'Origin');
  }
  if (req.method === 'OPTIONS') { res.status(204).end(); return; }
  next();
}

/* ---------------- 数据端口：申请 / 轮询 ---------------- */

function mountDataPortRoutes(app, { store, state }) {
  // 受配对总开关控制
  app.post('/_pair/request', (req, res) => {
    if (!state.pairingEnabled) {
      res.status(403).json({ error: 'forbidden', reason: 'pairing disabled' });
      return;
    }
    const deviceId = (req.body && req.body.device_id) || '';
    const name = (req.body && req.body.name) || '';
    const r = store.addPending(deviceId, name);
    if (r.error) {
      res.status(429).json({ error: r.error, reason: 'too many pending requests' });
      return;
    }
    console.log('[pair] 收到配对请求 device=' + deviceId + ' name=' + name + ' token=' + r.token);
    res.json({ token: r.token });
  });

  app.get('/_pair/status', (req, res) => {
    const s = store.status(req.query.token || '');
    if (s.state === 'notfound') {
      res.status(404).json({ error: 'not_found', reason: 'unknown token' });
      return;
    }
    res.json(s);   // { state:'pending' } 或 { state:'approved', user, pass }
  });
}

/* ---------------- 控制端口：待审 / 审批 / 开关 ---------------- */

function mountControlRoutes(app, { store, state, persistState }) {
  app.use(controlGuard);
  app.use(localCors);

  app.get('/pairing/status', (req, res) => res.json({ pairingEnabled: !!state.pairingEnabled }));

  app.post('/pairing/set', (req, res) => {
    state.pairingEnabled = !!(req.body && req.body.enabled);
    persistState(state);
    res.json({ ok: true, pairingEnabled: state.pairingEnabled });
  });

  app.get('/pair/requests', (req, res) => res.json({ requests: store.getPendingList() }));

  app.post('/pair/approve', (req, res) => {
    const r = store.approve((req.body && req.body.token) || '');
    if (!r) { res.status(404).json({ error: 'not_found', reason: 'unknown token' }); return; }
    if (r.error) { res.status(409).json({ error: r.error, reason: r.reason }); return; }
    res.json(r);   // { ok:true, user, pass }  ← 客户端必须校验 ok
  });

  app.post('/pair/deny', (req, res) => {
    res.json({ ok: store.deny((req.body && req.body.token) || '') });
  });
}

/* ---------------- 请求方：发起 + 轮询 ---------------- */

/**
 * 对端请求必须设超时：管理页会周期性轮询，对端离线时挂起的请求会不断累积，
 * 最终耗尽 fd/内存让事件循环卡死，表现为整个服务「假死」。
 */
function peerRequest(target, path, { method = 'GET', body = null, timeoutMs = 3000 } = {}) {
  return new Promise((resolve, reject) => {
    const payload = body ? JSON.stringify(body) : null;
    const req = http.request({
      host: target.host,
      port: target.port,
      path,
      method,
      headers: payload
        ? { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(payload) }
        : {},
    }, (res) => {
      const chunks = [];
      res.on('data', (c) => chunks.push(c));
      res.on('end', () => resolve({
        status: res.statusCode,
        json: safeJson(Buffer.concat(chunks).toString('utf8')),
      }));
    });
    req.setTimeout(timeoutMs, () => req.destroy(new Error('timeout')));
    req.on('error', reject);
    if (payload) req.write(payload);
    req.end();
  });
}

/** 发起配对：POST 申请 → 每 2s 轮询状态，直到 approved 或超时。 */
async function pairWith(target, { deviceId, name, waitMs = 60000, pollMs = 2000 } = {}) {
  const r = await peerRequest(target, '/_pair/request', {
    method: 'POST',
    body: { device_id: deviceId, name },
  });
  if (r.status === 403) throw new Error('对方未开启配对（pairing disabled）');
  if (r.status === 429) throw new Error('对方待审批请求过多，稍后重试');
  if (r.status !== 200 || !r.json || !r.json.token) throw new Error('配对申请失败');
  const token = r.json.token;

  const deadline = Date.now() + waitMs;
  while (Date.now() < deadline) {
    await sleep(pollMs);
    const s = await peerRequest(target, '/_pair/status?token=' + encodeURIComponent(token));
    if (s.status === 404) throw new Error('配对请求已失效');
    if (s.json && s.json.state === 'approved') {
      // 凭据落盘：key = deviceId（与发现列表的 key() 保持一致），UI 才能标「已配对」
      return { host: target.host, port: target.port, user: s.json.user, pass: s.json.pass };
    }
  }
  throw new Error('配对超时：对方未在 ' + Math.round(waitMs / 1000) + 's 内确认');
}

function safeJson(s) { try { return JSON.parse(s); } catch (_) { return null; } }
function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

module.exports = {
  mountDataPortRoutes,
  mountControlRoutes,
  peerRequest,
  pairWith,
  controlGuard,
  localCors,
  PairStore,
};
