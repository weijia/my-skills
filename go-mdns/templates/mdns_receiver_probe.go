// mdns_receiver_probe.go —— 诊断"Go 发现端收不到远程 mDNS"的对照工具。
//
// 背景：Windows 上 grandcat/zeroconf 绑 0.0.0.0/224.0.0.0 收不到远程响应
// （被 svchost Dnscache / msedge 抢占组播投递），必须"绑具体接口 IP + QU 查询"。
//
// 用法：
//   1) 把 ifaceIP 改成你自己的网卡 IP（如 192.168.1.50）
//   2) 把 remoteIP 改成你要检测的远程机器 IP（如 192.168.1.13）
//   3) go run mdns_receiver_probe.go
// 观察"来自远程的报文数"：绑接口 IP + QU 查询应稳定收到多条，绑通配则≈0。
package main

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	mcastIP  = "224.0.0.251" // mDNS 组播组
	ifaceIP  = "192.168.1.50" // ← 改成你的网卡 IP
	remoteIP = "192.168.1.13" // ← 改成你要检测的远程 IP
	port     = 5353
)

// 手写 PTR 查询 _nats._tcp.local. (QCLASS=0x8001 = QU 单播应答请求；去掉 0x80 变普通查询作对照)
var query = []byte{
	0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x05, '_', 'n', 'a', 't', 's', 0x04, '_', 't', 'c', 'p', 0x05, 'l', 'o', 'c', 'a', 'l', 0x00,
	0x00, 0x0c, 0x80, 0x01,
}

func main() {
	// 1) 绑"具体接口 IP:5353" + SO_REUSEADDR（bind 前）
	lc := net.ListenConfig{Control: func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			_ = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
		})
	}}
	p, err := lc.ListenPacket(nil, "udp4", net.JoinHostPort(ifaceIP, fmt.Sprint(port)))
	if err != nil {
		fmt.Println("bind/listen 失败:", err)
		os.Exit(1)
	}
	udp := p.(*net.UDPConn)
	defer udp.Close()

	// 2) bind 之后再 IP_ADD_MEMBERSHIP(组播组 + 接口 IP)；bind 前设置无效
	type ipMreq struct {
		multiaddr  [4]byte
		interface_ [4]byte
	}
	m := ipMreq{}
	copy(m.multiaddr[:], net.ParseIP(mcastIP).To4())
	copy(m.interface_[:], net.ParseIP(ifaceIP).To4())
	var joinErr error
	rc, err := udp.SyscallConn()
	if err != nil {
		fmt.Println("SyscallConn 失败:", err)
		os.Exit(1)
	}
	rc.Control(func(fd uintptr) {
		if e := syscall.Setsockopt(syscall.Handle(fd), syscall.IPPROTO_IP, 12, (*byte)(unsafe.Pointer(&m)), int32(unsafe.Sizeof(m))); e != nil {
			joinErr = e
		}
	})
	if joinErr != nil {
		fmt.Println("IP_ADD_MEMBERSHIP 失败:", joinErr)
		os.Exit(1)
	}
	fmt.Printf("[probe] 已绑定 %s, bind后 IP_ADD_MEMBERSHIP(%s + %s), 发 QU 查询\n", udp.LocalAddr(), mcastIP, ifaceIP)

	// 3) 密集发 QU 查询（每 500ms，共 8 次）
	dst := &net.UDPAddr{IP: net.ParseIP(mcastIP), Port: port}
	for i := 0; i < 8; i++ {
		_, _ = udp.WriteToUDP(query, dst)
		time.Sleep(500 * time.Millisecond)
	}

	// 4) 持续收包统计
	udp.SetReadDeadline(time.Now().Add(15 * time.Second))
	buf := make([]byte, 1500)
	total, remote := 0, 0
	for {
		n, src, err := udp.ReadFromUDP(buf)
		if err != nil {
			break
		}
		total++
		if src.IP.String() == remoteIP {
			remote++
		}
		fmt.Printf("[probe] src=%s:%d len=%d remote=%v\n", src.IP, src.Port, n, src.IP.String() == remoteIP)
	}
	fmt.Printf("[probe] 汇总: 共 %d 个报文, 来自远程 %s 的 %d 个\n", total, remoteIP, remote)
	if remote == 0 {
		fmt.Println("[probe] 收不到远程 → 检查: 是否绑了具体接口 IP？查询是否带 QU(0x8001)？防火墙？")
	} else {
		fmt.Println("[probe] 能收到远程 → 说明绑接口 IP + QU 有效，发现端应改为这种收包方式。")
	}
}
