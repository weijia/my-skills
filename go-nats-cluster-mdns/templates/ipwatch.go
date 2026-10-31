// ipwatch.go —— Windows NotifyIpInterfaceChange 事件驱动 IP 变化通知（非轮询）。
//
// 事件只报"何时变"，不报"变成什么"：收到事件后主循环仍需枚举一次接口地址
// （interfaceIPv4s()）比对，但这是"只在有变化时查一次"，不是周期轮询。
// 依赖：golang.org/x/sys v0.48.0（NotifyIpInterfaceChange/CancelMibChangeNotify2）。
package main

import (
	"context"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ipChangeFlag atomic.Bool

// 回调运行在系统 API 线程：只置 atomic 标志，严禁阻塞 / 分配 / 阻塞发 channel。
func onIpChange(caller unsafe.Pointer, row *windows.MibIpInterfaceRow, notif uint32) uintptr {
	ipChangeFlag.Store(true)
	return 0
}

// WatchIPChange 注册事件通知，返回"IP 变化事件"channel；ctx 取消时自动注销。
func WatchIPChange(ctx context.Context) (<-chan struct{}, error) {
	var h windows.Handle
	cb := windows.NewCallback(onIpChange)
	// AF_INET=2 只看 IPv4；initialNotification=false（启动时已有接口枚举作基准）
	if err := windows.NotifyIpInterfaceChange(windows.AF_INET, cb, nil, false, &h); err != nil {
		return nil, err
	}
	out := make(chan struct{}, 1)
	go func() {
		defer windows.CancelMibChangeNotify2(h)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			if ipChangeFlag.Swap(false) {
				// 非阻塞推送；已有未消费事件时丢弃新的（一次 IP 变化常触发多次回调）
				select {
				case out <- struct{}{}:
				default:
				}
			}
		}
	}()
	return out, nil
}
