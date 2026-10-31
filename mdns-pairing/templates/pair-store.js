'use strict';
/**
 * 配对 token 状态机（参考实现）
 *
 * 三个踩过的坑都体现在这里：
 *  P1 待审列表只返回 state==='pending'（否则点「允许」后那一行不消失）
 *  P2 已处理条目用短 TTL，且与待审批共用 CAP（否则反复配对后新请求被 429）
 *  P3 approve() 明确返回 ok，由调用方校验（HTTP 封装对 4xx 不抛异常）
 *
 * 接进项目：替换 persist/loadUsers 为自己的存储层即可；
 * 建议把 token 表单独放一个模块，别塞进用户表。
 */

const crypto = require('crypto');

const PENDING_CAP = 20;                    // 待处理总数上限（防刷）
const STALE_MS = 60 * 60 * 1000;           // 待审批：1 小时
const HANDLED_KEEP_MS = 5 * 60 * 1000;     // 已批准/已拒绝：只留够对方取凭据的时间

class PairStore {
  constructor({ users = {}, pending = {}, persist = () => {} } = {}) {
    this.users = users;       // { username: { passHash, role, deviceId, active } }
    this.pending = pending;   // { token: { type, deviceId, name, username, pass, state, createdAt } }
    this.persist = persist;
  }

  /** 生成一次性 token。error 时返回 { error } 由路由转成 429。 */
  addPending(deviceId, name, pass) {
    this.pruneStale();
    const active = Object.keys(this.pending)
      .filter((t) => this.pending[t].state === 'pending').length;
    if (active >= PENDING_CAP) {
      return { error: 'too_many', reason: 'too many pending requests' };
    }
    const token = crypto.randomBytes(16).toString('hex');
    this.pending[token] = {
      type: 'pair',
      deviceId: deviceId || '',
      name: name || '',
      username: 'peer-' + token.slice(0, 8),
      pass: pass || crypto.randomBytes(12).toString('hex'), // 明文留到对方取走为止
      state: 'pending',
      createdAt: Date.now(),
    };
    this.persist();
    return { token };
  }

  /** 审批通过。null=token 不存在；{error}=不可批准；否则 { ok, user, pass }。 */
  approve(token) {
    const p = this.pending[token];
    if (!p) return null;
    if (p.state !== 'pending') {
      return { error: 'already_handled', reason: 'request already handled' };
    }
    this.users[p.username] = {
      passHash: hashPassword(p.pass),
      role: 'peer',
      deviceId: p.deviceId || '',
      active: true,
      createdBy: 'pair',
      createdAt: Date.now(),
    };
    p.state = 'approved';   // 不删除：对方还要轮询 /_pair/status 取明文凭据
    this.persist();
    return { ok: true, user: p.username, pass: p.pass };
  }

  /** 拒绝：配对类可直接删除（对方轮询会拿到 notfound，UI 提示被拒绝）。 */
  deny(token) {
    const p = this.pending[token];
    if (!p) return false;
    delete this.pending[token];
    this.persist();
    return true;
  }

  /** 请求方轮询：pending / approved(含凭据) / notfound。 */
  status(token) {
    const p = this.pending[token];
    if (!p) return { state: 'notfound' };
    if (p.state === 'approved') {
      return { state: 'approved', user: p.username, pass: p.pass };
    }
    return { state: p.state || 'pending' };
  }

  /** 待审批列表：P1——必须过滤非 pending；注册类走各自的列表接口。 */
  getPendingList() {
    this.pruneStale();
    return Object.keys(this.pending)
      .filter((t) => this.pending[t].state === 'pending' && this.pending[t].type !== 'register')
      .map((t) => ({
        token: t,
        type: this.pending[t].type || 'pair',
        deviceId: this.pending[t].deviceId,
        name: this.pending[t].name,
        state: this.pending[t].state,
        createdAt: this.pending[t].createdAt,
      }))
      .sort((a, b) => b.createdAt - a.createdAt);
  }

  /** P2——已处理条目短 TTL；否则 1 小时后才清理，CAP 早被占满。 */
  pruneStale() {
    const now = Date.now();
    let changed = false;
    Object.keys(this.pending).forEach((t) => {
      const p = this.pending[t];
      const handled = p.state && p.state !== 'pending';
      const ttl = handled ? HANDLED_KEEP_MS : STALE_MS;
      if (now - p.createdAt > ttl) {
        delete this.pending[t];
        changed = true;
      }
    });
    if (changed) this.persist();
  }
}

function hashPassword(pass) {
  // 真实项目请用 scrypt/bcrypt；这里只做占位，避免示例引入依赖
  return crypto.createHash('sha256').update(String(pass)).digest('hex');
}

module.exports = { PairStore, PENDING_CAP, STALE_MS, HANDLED_KEEP_MS };
