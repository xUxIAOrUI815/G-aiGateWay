## 文件结构
g-aigateway/
├── cmd/
│   └── server/
│       └── main.go           # 程序入口，负责启动服务器和依赖注入
├── internal/                 # 核心私有逻辑（外部无法直接引用）
│   ├── proxy/                # 反向代理核心逻辑
│   ├── middleware/           # 限流、鉴权中间件
│   ├── ai/                   # AI 相关逻辑（语义缓存、Token统计）
│   └── config/               # 配置加载逻辑
├── pkg/                      # 公共工具包（如 Redis 连接池）
├── web/                      # (可选) 放一个 index.html 做简单的 Web 测试界面
├── go.mod                    # 项目依赖管理
├── .env                      # 存放 API Key 等敏感信息
└── docker-compose.yml        # 一键启动基础设施



## 请求流向

Main 启动监听。
RateLimit Middleware 拦截（检查 Redis Lua 脚本）。
Proxy.ServeHTTP 拦截。
检查 AICache。
命中：直接返回（闭环完成）。
未命中：执行 ReverseProxy.ServeHTTP。
Director 注入 API Key。
ReverseProxy 请求 DeepSeek。

## 项目亮点

虽然你现在用的是 sha256 匹配，但在面试中，你要准备好如下说法：
面试官： “你的缓存只是字符串匹配吗？”
你： “目前框架已打通。由于 Embedding 模型计算较慢且涉及额外开销，我在架构上预留了接口。生产环境下，我会调用 text-embedding 接口将 Prompt 转化为 1536 维向量，然后利用 Redis 的向量搜索功能或简单的余弦相似度公式进行阈值判断（比如相似度 > 0.95 视为命中）。这能大幅节省 Token 支出。”