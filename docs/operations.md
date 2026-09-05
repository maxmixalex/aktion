# 日常操作

## 启动与停止

```text
aktion up
aktion down
aktion restart
```

`up` 会解析所需 OCI Features、构建本地 runner image，然后启动每个 repository 的 listener。

## 查看状态

```text
aktion status
```

状态页显示 listener、runner、运行中的 job 和已分配但仍等待的 job。

其它视图：

```text
aktion repos
aktion runners
aktion stats
aktion logs
aktion logs <repo-id>
```

## 检查配置

```text
aktion doctor
aktion config
aktion config resolve <repo-id>
aktion features
```

`doctor` 检查本地配置、Docker、GitHub App 和 repository installation。

`config resolve` 输出某个 repository 最终使用的 base image、生成 image 和 Features。

## 清理 stale scale set

listener 异常退出后，GitHub 侧可能暂时留下同名 scale set。先确认没有重要 job 正在运行，然后：

```text
aktion cleanup
```

如果仍有 ephemeral runner，Aktion 默认拒绝 cleanup。确实需要强制处理时：

```text
aktion cleanup --force
```

只清理而不重新启动 listener：

```text
aktion cleanup --no-start
```
