package mdnshelper

// 模式 B：发现端（长连接客户端）—— 持续 Watch + 动态重定连接地址。
//
// 一次性 Discover(timeout) 只在调用那一刻扫一遍，地址变了它不知道。长连接客户端
// 应改用 Watch(持续浏览)，并在"地址集合变化"时把新地址喂给连接层重连，这样当
// 服务端因休眠唤醒/换网换了 IP，客户端也能自动跟到新地址。

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const entryTTL = 60 * time.Second

// Watch 持续浏览，地址集合（增/删/首次扫描完成）变化时回调 onUpdate（单 goroutine）。
func Watch(ctx context.Context, onUpdate func(urls []string)) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return fmt.Errorf("mdns resolver: %w", err)
	}
	entries := make(chan *zeroconf.ServiceEntry, 64)
	go func() { _ = resolver.Browse(ctx, ServiceType, ServiceDomain, entries) }()

	var (
		mu      sync.Mutex
		seen    = map[string]time.Time{}
		lastSet string
	)
	emit := func() {
		mu.Lock()
		now := time.Now()
		urls := make([]string, 0, len(seen))
		for u, t := range seen {
			if now.Sub(t) <= entryTTL {
				urls = append(urls, u)
			}
		}
		sort.Strings(urls)
		mu.Unlock()
		set := fmt.Sprintf("%v", urls)
		if set != lastSet {
			lastSet = set
			onUpdate(urls)
		}
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-entries:
			if !ok {
				return nil
			}
			if u := entryToURL(e); u != "" {
				mu.Lock()
				seen[u] = time.Now()
				mu.Unlock()
				emit()
			}
		case <-ticker.C: // 清理过期条目并视情况重发
			mu.Lock()
			now := time.Now()
			for u, t := range seen {
				if now.Sub(t) > entryTTL {
					delete(seen, u)
				}
			}
			mu.Unlock()
			emit()
		}
	}
}

// entryToURL 优先非回环 IPv4，构造 nats://host:port。
func entryToURL(e *zeroconf.ServiceEntry) string {
	if e == nil || e.Port <= 0 {
		return ""
	}
	var ip net.IP
	for _, a := range e.AddrIPv4 {
		if !a.IsLoopback() {
			ip = a
			break
		}
	}
	if ip == nil {
		for _, a := range e.AddrIPv6 {
			if !a.IsLoopback() {
				ip = a
				break
			}
		}
	}
	if ip == nil {
		return ""
	}
	return "nats://" + net.JoinHostPort(ip.String(), fmt.Sprintf("%d", e.Port))
}

// ---- 客户端接线示例（伪代码，接入你的连接层）----
//
// ctx, cancel := context.WithCancel(context.Background())
// defer cancel()
//
// // 初始一次性发现（拿启动用地址）
// if urls, err := Discover(3*time.Second); err == nil && len(urls) > 0 {
//     connLayer.SetBackupURL(urls[0]) // 记录/连接备服务器
// }
//
// // 持续 Watch：地址一变就重定连接
// go Watch(ctx, func(urls []string) {
//     if len(urls) > 0 {
//         connLayer.SetBackupURL(urls[0]) // 运行时重连到新地址（见下方契约）
//     }
// })
//
// 连接层需提供：
//   SetBackupURL(url string)：更新备地址；若备已连接则断开并按新地址重连；
//   若备是 active 后端，重连成功后重新订阅并触发 onConnect，业务恢复收消息。
