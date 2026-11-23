# 部署后排查手册（BK / Kubernetes）

> 构建期/历史故障见 [pitfalls.md](pitfalls.md)。

面向「包部署成功，但 Pod 不 Ready」的情况。按顺序执行，**不要从探针报错猜**。

## 一、先看三层状态

```bash
kubectl get pods -n <namespace> -o wide | grep <app>
kubectl describe pod <pod> -n <namespace> | tail -40
kubectl logs <pod> -n <namespace> -c <container> --tail=200
```

| 现象 | 含义 |
|---|---|
| `0/1 Running`，探针 `connection refused` | 进程活着，但没监听端口 |
| `CrashLoopBackOff` | 进程启动后退出，看 logs |
| `Running` 且 Ready | 已正常 |

## 二、看进程是否「活着但在等」

```bash
kubectl exec -it <pod> -n <namespace> -c <container> -- bash
```

```bash
ps -ef
cat /proc/<pid>/wchan; echo
grep -E 'State|Threads' /proc/<pid>/status
ls -l /proc/<pid>/fd
```

| 现象 | 含义 |
|---|---|
| `State: S` + `CPU TIME 00:00:00` | 卡在系统调用，不是慢启动 |
| `wchan=futex_wait_queue_me` | Go 运行时在等网络/锁 |
| fd 里只有日志、eventpoll | 还没有业务连接 |

## 三、看它连了谁（最有效的一步）

```bash
cat /proc/<pid>/net/tcp /proc/<pid>/net/tcp6
```

带翻译的版本：

```bash
python3 - <<'PY'
import socket
for f in ("/proc/net/tcp", "/proc/net/tcp6"):
    for line in open(f).read().splitlines()[1:]:
        p = line.split()
        def dec(a):
            ip, port = a.split(':')
            b = bytes.fromhex(ip)
            ip = socket.inet_ntoa(b[::-1]) if len(b) == 4 else socket.inet_ntop(socket.AF_INET6, b)
            return f"{ip}:{int(port,16)}"
        print(f.split('/')[-1], "local", dec(p[1]), "remote", dec(p[2]), "state", p[3])
PY
```

| 端口 | 依赖 |
|---|---|
| `0x670B` = 26379 | Redis Sentinel |
| `0x18EB` = 6379 | Redis |
| `0x6983` = 27017 | MongoDB |
| `0x0050` = 80 | 应用自身监听 |
| state `0A` | LISTEN |
| state `01` | ESTABLISHED |
| state `02` | SYN_SENT（卡在连接） |

**只有到 26379 的连接、没有到 6379/27017 的连接** → 卡在 Sentinel 阶段。

## 四、抓 goroutine 栈

```bash
kill -QUIT <pid>
kubectl logs <pod> -n <namespace> -c <container> --tail=200
```

日志里没有栈时，前台复跑一份：

```bash
timeout 30 /app/bin/<binary> <原参数> --addrport=0.0.0.0:8081 > /tmp/fg.log 2>&1 &
sleep 5
kill -QUIT $!
sleep 2
grep -A50 'goroutine 1 ' /tmp/fg.log | head -80
```

`goroutine 1` 是主流程，第一段调用栈就是卡点。

## 五、已验证案例：Redis 认证 + TLS

### 现象

- 进程存在、`0/1 Running`、探针 refused
- 日志停在业务初始化早期，无错误
- `net/tcp` 只有到 Sentinel `26379` 的 ESTABLISHED

### 原因 1：Sentinel 要求认证

```bash
exec 3<>/dev/tcp/<sentinel-host>/26379
printf 'PING\r\n' >&3
timeout 3 head -c 100 <&3
exec 3<&-
# -NOAUTH Authentication required.
```

应用模板变量是 `BK_CMDB_REDIS_SENTINEL_PASSWORD`，缺失时 `sentinelPwd` 为空，初始化阻塞。

### 原因 2：Redis 是明文，却注入了 TLS 变量

goroutine 栈：

```
crypto/tls.(*Conn).clientHandshake
github.com/gomodule/redigo/redis.Dial
```

说明 `BK_CMDB_REDIS_SSL_CERT_PATH` / `_KEY_PATH` / `_CA_CERT_PATH` 被设置，但 Redis 未启用 TLS，握手一直等待。

### 修法

1. 补 `BK_CMDB_REDIS_SENTINEL_PASSWORD`
2. 确认 Redis 是否 TLS：
   - 明文 → 删除三个 `BK_CMDB_REDIS_SSL_*`
   - TLS → 确保证书路径存在且有效

临时验证：

```bash
kubectl set env deployment/<deploy> -n <namespace> BK_CMDB_REDIS_SSL_CERT_PATH-
```

## 六、常见坑

| 现象 | 原因 | 处理 |
|---|---|---|
| 探针 30s 后失败 | 启动慢于探针窗口 | 调大 `initialDelaySeconds` / `failureThreshold` |
| 进程命令不是镜像 Entrypoint | BK 用 `app_desc.yaml` 的 `procCommand` | 两者保持一致 |
| `HOME` 与常规镜像不同 | 镜像设置了 `HOME=/app` | 对比正常 Pod 的 env；必要时显式覆盖 |
| 环境变量改了不生效 | 只重启 Pod 不会重读 app_desc | 重新部署；平台变量优先级高于 app_desc |
| 旧的 Pod 一直正常 | 老连接早已建立 | 重启旧 Pod 复现，才能对比 |

## 七、排查顺序总结

```
Pod 状态 → 进程状态(wchan/fd) → net/tcp 连了谁 → SIGQUIT 栈 → 对照配置/环境变量
```

前三步能覆盖大多数「进程活着但不服务」的问题；栈是最后也最准的证据。
