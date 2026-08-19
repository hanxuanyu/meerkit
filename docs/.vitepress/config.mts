import { defineConfig } from "vitepress";

export default defineConfig({
  lang: "zh-CN",
  title: "Meerkit",
  description: "Meerkit 监控服务使用、运维与插件开发文档",
  cleanUrls: true,
  lastUpdated: true,
  head: [
    ["link", { rel: "icon", type: "image/png", href: "/favicon.png" }],
    ["meta", { name: "theme-color", content: "#16856f" }],
  ],
  themeConfig: {
    logo: "/brand-mark.png",
    siteTitle: "Meerkit Docs",
    nav: [
      { text: "使用指南", link: "/guide/getting-started" },
      { text: "运维", link: "/operations/configuration" },
      { text: "插件开发", link: "/development/plugin-go" },
      { text: "参考", link: "/reference/cli" },
    ],
    sidebar: {
      "/guide/": [
        {
          text: "使用指南",
          items: [
            { text: "快速开始", link: "/guide/getting-started" },
            { text: "监控与条件", link: "/guide/monitoring" },
            { text: "通知", link: "/guide/notifications" },
            { text: "状态看板", link: "/guide/status-board" },
            { text: "浏览器执行节点", link: "/guide/browser-agent" },
            { text: "插件管理", link: "/guide/plugins" },
          ],
        },
      ],
      "/operations/": [
        {
          text: "运维指南",
          items: [
            { text: "配置", link: "/operations/configuration" },
            { text: "部署与升级", link: "/operations/deployment" },
            { text: "安全边界", link: "/operations/security" },
            { text: "日志与排障", link: "/operations/troubleshooting" },
          ],
        },
      ],
      "/development/": [
        {
          text: "项目开发",
          items: [
            { text: "架构与仓库", link: "/development/overview" },
            { text: "后端开发", link: "/development/backend" },
            { text: "前端开发", link: "/development/frontend" },
            { text: "浏览器自动化", link: "/development/browser-automation" },
          ],
        },
        {
          text: "插件开发",
          items: [
            { text: "Go 插件", link: "/development/plugin-go" },
            { text: "浏览器能力插件", link: "/development/browser-plugin" },
            { text: "跨语言协议", link: "/development/plugin-protocol" },
            { text: "一致性测试", link: "/development/plugin-testing" },
            { text: "打包与发布", link: "/development/releasing" },
          ],
        },
      ],
      "/reference/": [
        {
          text: "参考",
          items: [
            { text: "命令行", link: "/reference/cli" },
            { text: "HTTP API", link: "/reference/http-api" },
            { text: "浏览器 Action", link: "/reference/browser-actions" },
            { text: "插件清单", link: "/reference/plugin-manifest" },
            { text: "配置字段", link: "/reference/configuration" },
          ],
        },
      ],
    },
    socialLinks: [
      { icon: "github", link: "https://github.com/hanxuanyu/meerkit" },
    ],
    search: { provider: "local" },
    outline: { level: [2, 3], label: "本页目录" },
    docFooter: { prev: "上一页", next: "下一页" },
    lastUpdated: { text: "最后更新" },
    editLink: {
      pattern: "https://github.com/hanxuanyu/meerkit/edit/main/docs/:path",
      text: "在 GitHub 上编辑此页",
    },
    footer: {
      message: "基于 Apache License 2.0 开源",
      copyright: "Meerkit contributors",
    },
  },
});
