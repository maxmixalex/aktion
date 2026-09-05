# 设置 runner 数量与并发

Aktion 的容量限制是 **per repository** 的。理解这一点可以避免最常见的配置误会：`defaults.maxRunners` 只是每个 repository 继承的默认值，并不是整台 Docker 主机的全局并发上限。

## 默认缩到零

推荐的起点是：

```json
"defaults": {
  "minRunners": 0,
  "maxRunners": 1
}
```

没有 job 时，`minRunners: 0` 允许 runner 数量降到零，只保留轻量 listener。GitHub 分配 job 后，listener 再创建 runner container。

如果你希望某个 repository 始终保留一定 runner 容量，可以提高 `minRunners`：

```json
{
  "id": "latency-sensitive",
  "url": "https://github.com/OWNER/latency-sensitive",
  "enabled": true,
  "minRunners": 1,
  "maxRunners": 3
}
```

`minRunners` 和 `maxRunners` 不能为负数；当 `maxRunners` 大于 0 时，`minRunners` 不能超过它。

## 为 repository 单独提高上限

假设默认上限是 2：

```json
"defaults": {
  "maxRunners": 2
}
```

两个 repository 都继承这个值时，它们各自最多运行 2 个 runner，因此主机上理论上可以同时出现 4 个 runner。Aktion 当前没有额外的“整台主机最多 N 个 runner”总量限制。

需要更高并发的 repository 直接覆盖：

```json
{
  "id": "hpclang",
  "url": "https://github.com/OWNER/hpclang",
  "enabled": true,
  "maxRunners": 5
}
```

修改后用下面的命令确认最终值，而不是靠肉眼推断继承关系：

```text
aktion config resolve hpclang
```

输出中的 `minRunners` 和 `maxRunners` 就是这个 repository 实际传给 listener 的容量。

## 不设置上限

`maxRunners: 0` 在 Aktion 中表示 unlimited：

```json
{
  "id": "unlimited",
  "url": "https://github.com/OWNER/unlimited",
  "enabled": true,
  "maxRunners": 0
}
```

!!! warning "Unlimited 不等于无限资源"
    Aktion 不会在 `maxRunners: 0` 时替你控制主机的 CPU、内存或磁盘压力。大量并发 job 可以让 Docker 主机耗尽资源。只有在外部已经有容量约束，或者你明确接受这种风险时才使用 unlimited。

运行期间可以用 `aktion stats` 看当前 listener 与 runner container 的一次性 CPU/内存快照，用 `aktion status` 看每个 repository 的 runner、active job 和等待中的 job 数量。

## 选择合理上限

对编译、测试等 CPU 密集 workload，通常先从“主机能稳定承受的并发 job 数”开始，而不是从 CPU 线程数机械换算。不同 workflow 的内存和 I/O 差异可能很大，先把 `maxRunners` 设得保守，再结合 `aktion stats` 和真实构建耗时调整。
