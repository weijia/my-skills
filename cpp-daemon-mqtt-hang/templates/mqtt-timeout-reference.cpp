// cpp-daemon MQTT 挂起修复 —— 可套用的带超时代码模式
// 关键：任何网络操作都必须有有限超时；disconnect()->wait() 必须在锁外；
//       线程循环用 quit_ + cv_.wait_for 可中断。

#include <chrono>
#include <mqtt/async_client.h>

// 超时建议
constexpr auto CONNECT_TIMEOUT  = std::chrono::seconds(15);
constexpr auto DISCONNECT_TIMEOUT = std::chrono::seconds(5);
constexpr auto SUBSCRIBE_TIMEOUT = std::chrono::seconds(10);

// ---------- 1) 连接带超时（不要裸 connect().wait()） ----------
bool connectWithTimeout(mqtt::async_client& cli, mqtt::connect_options& opts) {
  try {
    // wait_for 返回 true 表示完成，false 表示超时
    bool done = cli.connect(opts)->wait_for(CONNECT_TIMEOUT);
    if (!done) {
      // 超时：主动断开并返回失败，让上层重试/退出
      cli.disconnect();
      return false;
    }
    return true;
  } catch (const std::exception& e) {
    return false;
  }
}

// ---------- 2) 断开带超时，且必须在锁外调用 ----------
void disconnectOutsideLock(std::unique_ptr<mqtt::async_client>& cli) {
  if (!cli) return;
  try {
    // 持锁时绝不要 wait()：会与 paho 内部网络线程争锁，导致 join() 卡死
    cli->disconnect()->wait_for(DISCONNECT_TIMEOUT);
  } catch (...) {}
  {
    // 锁内只做 reset，不做任何 wait
    // std::lock_guard<std::mutex> lk(cliMu_);
    cli.reset();
  }
}

// ---------- 3) 线程循环可中断（quit_ + cv_.wait_for） ----------
// std::thread loop([this] {
//   // 连接失败按 interval 重试，直到成功或 quit
//   while (!quit_.load() && !connectWithTimeout(*cli_, opts)) {
//     std::unique_lock<std::mutex> lk(cvMu_);
//     cv_.wait_for(lk, std::chrono::seconds(interval), [this]{ return quit_.load(); });
//   }
//   if (quit_.load()) return;
//
//   // 发布循环：不阻塞
//   while (!quit_.load()) {
//     {
//       std::unique_lock<std::mutex> lk(cvMu_);
//       cv_.wait_for(lk, std::chrono::seconds(interval), [this]{ return quit_.load(); });
//     }
//     if (quit_.load()) break;
//     // cli_->publish(...) 异步，不走 wait
//   }
//
//   // 收尾：锁外 disconnect()->wait()
//   disconnectOutsideLock(cli_);
// });

// ---------- 4) shutdown()：置 quit_ → notify → join ----------
// void shutdown() override {
//   quit_ = true;
//   cv_.notify_all();
//   {
//     std::lock_guard<std::mutex> lk(cliMu_);
//     if (cli_) { try { cli_->disconnect(); } catch (...) {} }  // 触发断开（非阻塞）
//   }
//   if (thread_.joinable()) thread_.join();
//   {  // join 后，锁内 reset
//     std::lock_guard<std::mutex> lk(cliMu_);
//     if (cli_) { try { cli_->disconnect()->wait_for(DISCONNECT_TIMEOUT); } catch (...) {} cli_.reset(); }
//   }
// }

// ---------- 5) HTTP 上传（httplib）带超时 ----------
// httplib::Client cli(host, port);
// cli.set_connection_timeout(10, 0);   // 10s
// cli.set_read_timeout(15, 0);         // 15s
// cli.set_write_timeout(10, 0);        // 10s
