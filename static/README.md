# RSS Reader - 仿报纸风格 RSS 阅读器

一个仿报纸排版风格的 RSS 阅读器，支持 Docker 部署，SQLite 存储，带后台管理界面。

基于 [silky/news](https://github.com/silky/news) 改造。

## 功能特性

- 📰 仿报纸排版的首页展示
- 🌙 深色/浅色主题切换
- 📱 响应式设计，支持移动端
- 🔧 后台管理 RSS 订阅源
- 📝 后台管理文章（查看/删除/按源批量删除）
- 🔄 手动触发 RSS 抓取
- 💾 SQLite 数据存储
- 🐳 Docker 一键部署

## 快速开始

### 方式一：Docker Compose（推荐）

1. 修改 `docker-compose.yml` 中的管理员密码：

```yaml
environment:
  - ADMIN_PASSWORD=你的密码
```

2. 启动服务：

```bash
docker-compose up -d
```

3. 访问：
   - 前台：http://你的IP:8080
   - 后台：http://你的IP:8080/admin/

### 方式二：Docker 构建

```bash
docker build -t rss-reader .
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e ADMIN_PASSWORD=你的密码 \
  --name rss-reader \
  rss-reader
```

### 方式三：本地编译运行

需要 Go 1.22+

```bash
go mod tidy
go build -o news .
ADMIN_PASSWORD=你的密码 ./news
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `ADMIN_PASSWORD` | `admin123` | 后台管理密码 |
| `DB_PATH` | `news.db` | SQLite 数据库文件路径 |
| `PORT` | `8080` | 服务端口 |
| `TZ` | `Asia/Shanghai` | 时区 |

## 后台管理

访问 `/admin/` 进入后台管理界面。

### 功能

- **RSS 源管理**：添加、编辑、删除、启用/禁用 RSS 源
- **文章管理**：浏览文章列表、按来源筛选、删除文章、按源批量删除
- **手动抓取**：点击按钮立即抓取所有启用的 RSS 源

### 默认 RSS 源

预置了澎湃新闻的 RSS 源（通过 RSSHub），可在后台管理界面添加更多源。

常用 RSSHub 路由示例：
- 澎湃新闻：`https://rsshub.app/thepaper/newsDetail`
- 36氪：`https://rsshub.app/36kr/newsflashes`
- 少数派：`https://rsshub.app/sspai/columns`
- IT之家：`https://rsshub.app/ithome/ranking`

更多路由请参考 [RSSHub 文档](https://docs.rsshub.app)

## 数据持久化

SQLite 数据库文件默认存储在容器内的 `/data/news.db`，通过 Docker volume 挂载到宿主机的 `./data/` 目录。

## 飞牛 NAS 部署说明

1. 通过 SSH 登录 NAS
2. 将项目文件夹上传到 NAS
3. 进入项目目录，执行：
   ```bash
   docker-compose up -d
   ```
4. 访问 `http://NAS_IP:8080`

如需修改端口，编辑 `docker-compose.yml` 中的 `ports` 配置。

## 自定义

### 添加更多 RSS 源

登录后台管理界面（`/admin/`），在 RSS 源管理中添加。

### 修改主题

页面右上角 Aa 按钮可切换：
- 亮色/暗色主题
- Serif/Sans 字体
- 大/小字号

## 许可证

AGPL-3.0（继承原项目）
