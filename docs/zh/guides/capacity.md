# 配置 runner 数量和并发

Aktion 的容量限制按仓库分别计算。`defaults.maxRunners` 是各仓库继承的默认值，不是 Docker 主机的全局并发上限。

## 空闲时缩容到零

推荐从以下配置开始：

```json
"defaults": {
  "minRunners": 0,
  "maxRunners": 1
}
```

设置 `minRunners: 0` 后，没有作业时该仓库可以保持零个 runner，只保留轻量的 listener。GitHub 分配作业后，listener 再创建 runner。

如果某个仓库需要降低 runner 启动带来的等待时间，可以保留一定的最低容量：

```json
{
  "id": "latency-sensitive",
  "url": "https://github.com/OWNER/latency-sensitive",
  "enabled": true,
  "minRunners": 1,
  "maxRunners": 3
}
```

`minRunners` 和 `maxRunners` 都不能为负数。当 `maxRunners` 大于 0 时，`minRunners` 不能大于 `maxRunners`。

## 单独提高某个仓库的上限

假设默认值为：

```json
"defaults": {
  "maxRunners": 2
}
```

两个仓库都继承该值时，每个仓库最多可以运行两个 runner，因此主机理论上可同时运行四个 runner。Aktion 当前没有额外的全主机 runner 总量限制。

需要更高并发的仓库可以单独覆盖该值：

```json
{
  "id": "hpclang",
  "url": "https://github.com/OWNER/hpclang",
  "enabled": true,
  "maxRunners": 5
}
```

修改后运行以下命令，确认最终解析结果：

```text
aktion config resolve hpclang
```

输出中的 `minRunners` 和 `maxRunners` 就是 listener 实际使用的容量设置。

## 取消 runner 上限

在 Aktion 中，`maxRunners: 0` 表示不设置 runner 数量上限：

```json
{
  "id": "unlimited",
  "url": "https://github.com/OWNER/unlimited",
  "enabled": true,
  "maxRunners": 0
}
```

!!! warning "不设上限并不代表主机资源无限"
    使用 `maxRunners: 0` 时，Aktion 不会额外限制 CPU、内存或磁盘用量。作业集中到达时，runner 数量可能迅速增加并耗尽 Docker 主机资源。仅在已有其他容量限制，或能够接受这一风险时使用该设置。

运行期间，`aktion stats` 可显示 listener 和 runner 容器当前的 CPU、内存快照；`aktion status` 可显示 runner、正在执行的作业，以及已分配但仍在等待的作业。

## 选择合适的上限

对于编译和测试等 CPU 密集型工作负载，应根据主机能够稳定承受的并发作业数设置上限，而不是简单按 CPU 线程数换算 runner 数量。不同工作流的内存和 I/O 需求可能差异很大。建议先设置较低的上限，再结合实际构建耗时和 `aktion stats` 的结果调整。
