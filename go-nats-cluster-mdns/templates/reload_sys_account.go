// reload_sys_account.go —— 用 NATS 系统账号触发热重载（替代 Windows 非 service 下失效的 --signal）。
//
// Windows 上 nats-server 以普通 exe 进程运行时，`nats-server --signal reload`
// 无法工作（那是 service 专用通道，报 Access denied）。正确方式：通过系统账号发
// NATS 请求触发 reload：$SYS.REQ.SERVER.PING 拿 server_id →
// $SYS.REQ.SERVER.<id>.RELOAD（NATS ≥2.10）。零第三方依赖，离线可编。
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// sysReload 连接 server，用系统账号 user/pass 触发 reload。
func sysReload(serverURL, user, pass string) error {
	host := serverURL
	if u, err := url.Parse(serverURL); err == nil && u.Host != "" {
		host = u.Host
	}
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		return fmt.Errorf("连接 %s 失败: %w", host, err)
	}
	defer conn.Close()
	br := bufio.NewReader(conn)

	// 读 server 首条 INFO
	line, err := br.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读 INFO 失败: %w", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(line), "INFO") {
		return fmt.Errorf("预期 INFO，实际: %s", strings.TrimSpace(line))
	}

	// CONNECT（系统账号认证）
	connect := fmt.Sprintf("CONNECT {\"user\":%q,\"pass\":%q,\"verbose\":false,\"pedantic\":false,\"name\":\"nats-cluster-mdns\",\"lang\":\"go\",\"version\":\"1.0.0\"}\r\n", user, pass)
	if _, err := conn.Write([]byte(connect)); err != nil {
		return err
	}

	// 订阅 inbox 收系统请求响应
	inbox := "_INBOX." + randToken()
	if _, err := conn.Write([]byte(fmt.Sprintf("SUB %s 1\r\n", inbox))); err != nil {
		return err
	}

	// 1) PING 拿本机 server id
	if _, err := conn.Write([]byte(fmt.Sprintf("PUB $SYS.REQ.SERVER.PING %s 0\r\n\r\n", inbox))); err != nil {
		return err
	}
	pingPayload, err := readReply(conn, br, inbox, 4*time.Second)
	if err != nil {
		return fmt.Errorf("系统账号 PING 失败（请确认 config 已配置 system_account 且账号可用）: %w", err)
	}
	var pr struct {
		Server struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"server"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(pingPayload, &pr); err != nil {
		return fmt.Errorf("解析 PING 响应失败: %v (payload=%s)", err, string(pingPayload))
	}
	if pr.Error != "" {
		return fmt.Errorf("系统账号 PING 返回错误: %s", pr.Error)
	}
	if pr.Server.ID == "" {
		return fmt.Errorf("未从 PING 响应拿到 server id: %s", string(pingPayload))
	}

	// 2) 发 reload 请求（v2.10+ 需带 server id）
	reloadSubj := "$SYS.REQ.SERVER." + pr.Server.ID + ".RELOAD"
	body := "{}"
	pub := fmt.Sprintf("PUB %s %s %d\r\n%s\r\n", reloadSubj, inbox, len(body), body)
	if _, err := conn.Write([]byte(pub)); err != nil {
		return err
	}
	reloadPayload, err := readReply(conn, br, inbox, 6*time.Second)
	if err != nil {
		return fmt.Errorf("reload 请求无响应: %w", err)
	}
	var rp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(reloadPayload, &rp); err != nil {
		return fmt.Errorf("解析 reload 响应失败: %v (payload=%s)", err, string(reloadPayload))
	}
	if rp.Error != "" {
		return fmt.Errorf("reload 失败: %s", rp.Error)
	}
	return nil
}

// readReply 从连接读 MSG，返回匹配 inbox 的 payload。
func readReply(conn net.Conn, br *bufio.Reader, inbox string, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		conn.SetReadDeadline(deadline)
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "MSG ") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		subj := f[1]
		n, err := strconv.Atoi(f[len(f)-1])
		if err != nil {
			continue
		}
		if subj != inbox {
			_, _ = io.CopyN(io.Discard, br, int64(n)+2) // 丢弃非目标消息 + \r\n
			continue
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(br, buf); err != nil {
			return nil, err
		}
		_, _ = br.Discard(2)
		return buf, nil
	}
}

func randToken() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
