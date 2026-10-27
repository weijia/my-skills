package mdnshelper

// 模式 A：广播端 —— 让 mDNS 在"休眠唤醒 / 切换网络换 IP"后仍然广播正确地址。
//
// 关键坑：grandcat/zeroconf 的 Register() 在注册那一刻把接口 IP 写死进
// entry.AddrIPv4/AddrIPv6；其 appendAddrs 仅在缓存地址为空时才回退到实时接口，
// 因此换 IP 后它会一直用旧 IP 应答。解决办法是轮询 net.InterfaceAddrs()，
// 一旦发现地址集合变化就：对旧 responder 发 goodbye(TTL=0) + 重新 Register
// （新 responder 的宣告带 cache-flush，对端立即替换旧 IP）。

import (
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	ServiceType    = "_nats._tcp"
	ServiceDomain  = "local."
	ipPollInterval = 5 * time.Second
)

type Advertiser struct {
	mu       sync.Mutex
	srv      *zeroconf.Server
	instance string
	port     int
	txt      []string
}

func Advertise(ctx context.Context, instance string, port int, txt []string) (*Advertiser, error) {
	if instance == "" {
		instance = "nats-server"
	}
	srv, err := zeroconf.Register(instance, ServiceType, ServiceDomain, port, txt, nil)
	if err != nil {
		return nil, fmt.Errorf("mdns register: %w", err)
	}
	a := &Advertiser{srv: srv, instance: instance, port: port, txt: txt}
	go a.ipWatch(ctx) // 核心：地址变化自动重广播
	go func() {
		<-ctx.Done()
		a.Shutdown()
	}()
	return a, nil
}

func (a *Advertiser) Shutdown() {
	a.mu.Lock()
	srv := a.srv
	a.srv = nil
	a.mu.Unlock()
	if srv != nil {
		srv.Shutdown() // 内部会发 TTL=0 goodbye
	}
}

func (a *Advertiser) ipWatch(ctx context.Context) {
	last := currentGlobalAddrs()
	ticker := time.NewTicker(ipPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cur := currentGlobalAddrs()
			if sameStringSlices(last, cur) {
				continue
			}
			log.Printf("[mdns] 本地地址变化，重新广播 (旧=%v 新=%v)", last, cur)
			a.reRegister()
			last = cur
		}
	}
}

func (a *Advertiser) reRegister() {
	a.mu.Lock()
	old := a.srv
	a.mu.Unlock()
	if old != nil {
		old.Shutdown() // goodbye TTL=0：对端尽快丢弃旧地址
	}
	// 给 goodbye 发出去留时间，避免重 probe 时瞬间重名冲突
	time.Sleep(250 * time.Millisecond)
	srv, err := zeroconf.Register(a.instance, ServiceType, ServiceDomain, a.port, a.txt, nil)
	if err != nil {
		log.Printf("[mdns] 重新广播失败: %v", err)
		return
	}
	a.mu.Lock()
	a.srv = srv
	a.mu.Unlock()
}

// currentGlobalAddrs 返回所有 up 接口的非回环全局单播 IP（排序），用于检测地址变化。
// 用轮询而非 OS 事件：net.InterfaceAddrs 在 Win/Mac/Linux 一致可用，跨平台最简单。
func currentGlobalAddrs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]string, 0, 4)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP
			if ip.IsLoopback() {
				continue
			}
			if ip.To4() != nil || ip.IsGlobalUnicast() {
				out = append(out, ip.String())
			}
		}
	}
	sort.Strings(out)
	return out
}

func sameStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
