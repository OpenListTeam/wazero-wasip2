# wazero-wasip2

该库为 [wazero](https://wazero.io/) 运行时提供了 WASI Preview 2 (WASIp2) 的实现。它通过实现 WASI 预览2 中定义的接口，使得遵循该标准的 WebAssembly 模块能够在 wazero 环境中与系统资源（如网络、文件系统等）进行交互。
需要注意的是虽然提供了必要的WASIp2接口，但是wazero只支持wasip1模式编译，并不支持WASIp2模式


## 待解决问题

1.  HTTP 请求有概率丢失 body，将 `Flush` 改为同步后问题有所缓解。需要审查完整的 HTTP 流程。
2.  实现 HTTP 的超时设置。
3.  `wazero-wasip2` 目前只完成了部分验证，需要编写测试用例进行审查（重点：资源释放）。现已完成 `clock`, `http` (guest->host), `io`, `tls`, `random` 的部分审查。