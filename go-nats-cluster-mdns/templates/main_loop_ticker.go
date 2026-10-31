// main_loop_ticker.go —— 守护进程主循环骨架：Ticker 汇总（避免 timer 饿死）
// + discovered TTL 清理 + IP 变化事件重建广播/发现。
//
// 关键：汇总必须用 time.NewTicker（跨循环保持），不能写 select 里的
// `case <-time.After(checkEvery)`——那每次进 select 都重建 timer，网内持续
// mDNS 广播（entries 频繁 continue）时循环快速重进 select、timer 永远被重置，
// 汇总/写盘/reload 会被饿死。
package main

import (
	"context"
	"log"
	"time"
)

func mainLoop(ctx context.Context, o *options) error {
	// 广播（返回 *RouteAdvertiser，IP 变化时 Shutdown 后重新 Register）
	adv, err := AdvertiseRoute(ctx, o.serverName, o.routePort, o.cluster, o.serverName)
	if err != nil {
		return err
	}
	// 发现（返回 *RouteDiscovery，IP 变化时 Close 后重新 NewRouteDiscovery）
	disc, err := NewRouteDiscovery(o.cluster)
	if err != nil {
		return err
	}
	entries := make(chan routeEntry, 64)
	go disc.Serve(ctx, entries)

	// 退出时回收"最新"的广播/发现（重建后变量被重新赋值，闭包捕获最新值）
	defer func() { adv.Shutdown(); disc.Close() }()

	// IP 变化事件（Windows NotifyIpInterfaceChange，非轮询）
	ipEvents, err := WatchIPChange(ctx)
	if err != nil {
		return err
	}
	baseIPs := interfaceIPv4s() // 启动基准，与 disc.selfIPs 一致

	const discoverTTL = 30 * time.Second // 对端停止广播后淘汰其旧地址
	discovered := map[string]bool{}
	discoveredTTL := map[string]time.Time{}
	dirty := false
	lastReload := time.Time{}
	ticker := time.NewTicker(o.checkEvery)
	defer ticker.Stop()

	for {
		select {
		case e := <-entries:
			for _, r := range e.routes() {
				discovered[r] = true
				discoveredTTL[r] = time.Now().Add(discoverTTL)
			}
			continue
		case <-ipEvents:
			// 事件驱动（非轮询）：收到后枚举一次比对；debounce 合并多次回调
			debounceIP(ipEvents)
			cur := interfaceIPv4s()
			if ipSetEqual(baseIPs, cur) {
				continue // 事件到达但 IPv4 集合没变，忽略
			}
			baseIPs = cur
			// 1) 重建广播：用新接口 IP 重新 announce（否则对端仍收到旧地址）
			adv.Shutdown()
			adv, err = AdvertiseRoute(ctx, o.serverName, o.routePort, o.cluster, o.serverName)
			if err != nil {
				log.Printf("IP 变化后重新广播失败: %v", err)
			}
			// 2) 重建发现端（新 selfIPs + 新 socket），旧 goroutine 由 Close 回收
			disc.Close()
			disc, err = NewRouteDiscovery(o.cluster)
			if err != nil {
				log.Printf("IP 变化后重建发现端失败: %v", err)
			} else {
				go disc.Serve(ctx, entries)
			}
			// 3) 清空旧 discovered，强制下一轮用新发现重写 routes 并 reload
			discovered = map[string]bool{}
			discoveredTTL = map[string]time.Time{}
			dirty = true
			continue
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		// 汇总前清理过期发现地址
		now := time.Now()
		for r, exp := range discoveredTTL {
			if now.After(exp) {
				delete(discovered, r)
				delete(discoveredTTL, r)
			}
		}

		routes := buildRoutes(discovered, o.seed)
		changed, err := updateRoutesFile(o.routesFile, routes)
		if err != nil {
			log.Printf("写 routes 文件失败: %v", err)
			continue
		}
		if changed {
			dirty = true
			log.Printf("路由变化，标记待 reload: %d 条", len(routes))
		}
		if dirty && time.Since(lastReload) >= o.reloadWindow {
			if err := validateConfig(o.natsServer, o.config); err != nil {
				log.Printf("reload 跳过: %v", err)
				continue
			}
			if err := sysReload(o.sysURL, o.sysUser, o.sysPass); err != nil {
				log.Printf("reload 失败: %v", err)
				continue
			}
			dirty = false
			lastReload = time.Now()
			log.Printf("已 reload，应用 %d 条 route", len(routes))
		}
	}
}

// debounceIP 合并一次 IP 变化触发的连续事件（最多等 400ms），避免频繁重建。
func debounceIP(ch <-chan struct{}) {
	t := time.NewTimer(400 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ch:
		case <-t.C:
			return
		}
	}
}

// ipSetEqual 比较两个 IPv4 集合是否相等（忽略顺序）。
func ipSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]bool, len(a))
	for _, x := range a {
		m[x] = true
	}
	for _, x := range b {
		if !m[x] {
			return false
		}
	}
	return true
}

// buildRoutes 合并 discovered + seed 为 route 列表（去重）。
func buildRoutes(discovered map[string]bool, seed []string) []string {
	routes := make([]string, 0, len(discovered)+len(seed))
	seen := map[string]bool{}
	for r := range discovered {
		if !seen[r] {
			seen[r] = true
			routes = append(routes, r)
		}
	}
	for _, r := range seed {
		if !seen[r] {
			seen[r] = true
			routes = append(routes, r)
		}
	}
	return routes
}
