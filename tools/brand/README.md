# 品牌配置

`settings.json` 是本站的品牌配置（站点名、头像、页脚、菜单、首页顶部），本地和服务器复用同一份。

导入：后台「系统设置」→ 配置导入，上传 `settings.json`。只覆盖文件里列出的键，其他配置不受影响。

不放在这里的：`SITE_URL`（每个环境不同：本地 http://localhost:3000，线上 https://thpvip.cn），在后台单独设置。

## 导入后还要在后台手动做的（每个新环境做一次）

首个账号注册时写入的初始化数据不属于系统设置，导入 `settings.json` 不会处理：

1. **系统设置 → 站点信息**：`SITE_URL` 设为当前环境地址（不改的话 canonical、og:url 都指向 anheyu.com）
2. **文章管理**：删除默认文章「欢迎使用 Anheyu-App！」
3. **友链管理**：删除两条「安知鱼」友链
4. **页面管理**：
   - 隐私政策：`blog.anheyu.com` 改为 `thpvip.cn`
   - Cookie 政策：两处「请通过 `anzhiyu-c@qq.com` 与我联系」改为「请通过本站留言与我联系」
   - 版权声明：删除「关于主题样式的版权信息，可以详见 安和鱼应用说明」这一段
5. **系统设置 → 文章配置 → 打赏配置**：默认是原作者的支付宝/微信收款码，**上线前必须换成自己的或关闭打赏**
6. 跑 `docs/plans/2026-10-11-blog-migration/04-branding.md` Task R4 Step 3 的残留检查脚本，应无输出

已知保留项：`footer.badge.list` 里仍是原作者的徽章，但 `footer.badge.enable` 为 `false` 不显示，开启前先改；表情包、技能图标、ESFJ 性格图标引用的 CDN 地址里带 `anzhiyu`，只是资源地址，前台不显示。
