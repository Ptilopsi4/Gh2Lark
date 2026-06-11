# ==================================================
# Gh2Lark 部署步骤
# ==================================================

## 1️⃣ 服务器准备（首次）

# SSH 登录
ssh root@your-vps-ip

# 安装 Docker（如未安装）
curl -fsSL https://get.docker.com | sh
systemctl enable docker --now

# 克隆仓库
git clone https://github.com/Ptilopsi4/Gh2Lark.git /opt/gh2lark
cd /opt/gh2lark

## 2️⃣ 配置环境变量

# 创建 .env 文件
cat > .env << 'EOF'
LARK_WEBHOOK_URL=https://open.feishu.cn/open-apis/bot/v2/hook/YOUR-HOOK-ID
LARK_SIGNING_SECRET=your-bot-signing-secret
GITHUB_WEBHOOK_SECRET=your-webhook-secret
EOF

# ⚠️ 把 YOUR-HOOK-ID 和 secret 换成真实值
# LARK_SIGNING_SECRET 来自飞书机器人安全设置页面的「签名校验」密钥
# 如果机器人未开启签名校验，可留空或删除这一行

## 3️⃣ 构建并启动

# 构建 + 后台启动
docker compose up -d --build

# 确认运行
docker compose ps
docker compose logs -f --tail=50

# 自检
curl http://localhost:8080/health
# → {"status":"ok"}

## 4️⃣ 配置反向代理 (HTTPS)

### 方案 A — Nginx（推荐，如果已有）

# 添加 site config: /etc/nginx/sites-available/gh2lark
cat > /etc/nginx/sites-available/gh2lark << 'NGINX'
server {
    listen 443 ssl http2;
    server_name hooks.your-domain.com;

    # SSL 证书路径（按你现有配置）
    ssl_certificate     /etc/letsencrypt/live/hooks.your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/hooks.your-domain.com/privkey.pem;

    location /webhook {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # GitHub 会发 25MB payload，超时设大些
        proxy_read_timeout 30s;
        client_max_body_size 30m;
    }

    location /health {
        proxy_pass http://127.0.0.1:8080;
    }
}
NGINX

# 启用
ln -sf /etc/nginx/sites-available/gh2lark /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx

# 申请 SSL 证书（如未申请）
certbot --nginx -d hooks.your-domain.com

### 方案 B — Caddy（自动 TLS）

# /etc/caddy/Caddyfile
cat >> /etc/caddy/Caddyfile << 'CADDY'
hooks.your-domain.com {
    reverse_proxy /webhook localhost:8080
    reverse_proxy /health  localhost:8080
}
CADDY

systemctl reload caddy

## 5️⃣ GitHub Webhook 设置

# 1. 打开仓库 Settings → Webhooks → Add webhook
# 2. Payload URL:    https://hooks.your-domain.com/webhook
# 3. Content type:   application/json
# 4. Secret:         与 .env 中 GITHUB_WEBHOOK_SECRET 一致
# 5. SSL verification: Enable
# 6. Events:         Let me select → Push, Pull requests, Issues
# 7. Active:         ✅

## 6️⃣ 更新代码（后续）

ssh root@your-vps "cd /opt/gh2lark && git pull && docker compose up -d --build"

## ==================================================
# 健康检查 & 故障排查
# ==================================================

# 查看实时日志
docker compose -f /opt/gh2lark/docker-compose.yml logs -f

# 手动触发测试
curl -X POST https://hooks.your-domain.com/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: ping" \
  -d '{"zen":"hello"}'

# GitHub 也会在 Webhook 设置页面提供 "Redeliver" 按钮
# 用于重放真实事件做调试
