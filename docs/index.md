---
layout: home

hero:
  name: Meerkit
  text: 自托管监控服务
  tagline: 用独立插件执行探测，用统一的调度、条件、通知和状态看板组织结果。
  image:
    src: /meerkit.png
    alt: Meerkit
  actions:
    - theme: brand
      text: 开始使用
      link: /guide/getting-started
    - theme: alt
      text: 开发插件
      link: /development/plugin-go

features:
  - title: 监控与条件
    details: 定时或手动执行 HTTP、TCP 监控，比较当前与历史结果，并记录触发和恢复状态。
    link: /guide/monitoring
    linkText: 配置监控
  - title: 通知与看板
    details: 使用站内、Webhook、SMTP 渠道接收事件，通过状态看板观察字段、阈值和趋势。
    link: /guide/notifications
    linkText: 管理通知
  - title: 可扩展插件
    details: 官方插件使用 Go SDK；第三方语言可以实现公开协议，浏览器插件还能复用平台管理的 Chrome 执行能力。
    link: /development/plugin-protocol
    linkText: 阅读协议
---

## 文档范围

本网站描述仓库当前已经实现的功能，不作为未来路线图。使用者可以从[快速开始](/guide/getting-started)进入；需要真实浏览器采集时阅读[浏览器执行节点](/guide/browser-agent)；部署维护者应重点阅读[配置](/operations/configuration)、[部署与升级](/operations/deployment)和[安全边界](/operations/security)；插件作者可以选择 [Go SDK](/development/plugin-go) 或[跨语言协议](/development/plugin-protocol)。

Meerkit 主项目、官方插件、SDK 和本文档均以 [Apache License 2.0](https://github.com/hanxuanyu/meerkit/blob/main/LICENSE) 开源。
