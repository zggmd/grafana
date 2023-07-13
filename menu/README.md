# 微前端开发指南

## 本地首次开发

### 安装依赖 & 开启本地服务

按照 [developer-guide.md](https://github.com/grafana/grafana/blob/HEAD/contribute/developer-guide.md)
进行相关安装依赖 & 开启本地服务

服务起来后,本地地址将起在 http://localhost:3000

### 本地安装依赖不成功的问题

`yarn install --immutable ` 可能失败

建议全局翻墙安装

### 获取 menu-generator

进入 `grafana 代码仓库根目录`执行:

```shell
mkdir menu-generator && \
cd menu-generator && \
git init && \
git remote add -f origin https://github.com/bestchains/bc-console.git && \
git config core.sparsecheckout true && \
echo "config/menu/menu-generator" >> .git/info/sparse-checkout && \
git checkout main && \
cd ..
```

将在项目根目录增加 `menu-genarator`, 作为菜单的构建工具

### 执行构建菜单

为了不引入新的依赖, 请在新终端手动执行:

```shell
yarn build:menu
```

将生成菜单文件, 并被托管在地址: http://localhost:3000/public/build/menu.json ,下面有用

### 从 qiankun 主应用中加载 grafana

#### 解决跨域问题

主应用中加载本地的 grafana, 会有跨域问题,所以需要一些设置:

##### 安装 xswitch

chrome 安装工具: [xswitch](https://chrome.google.com/webstore/detail/xswitch/idkjhjggpffolpidfkikidcokdkdaogg)

##### 配置 xswitch

点开 chrome 的 xswitch 图标,增加如下配置:

```json lines
{
  // ....
  // urls that want CORS
  "cors": [
    // ...
    "localhost:3000" // 新增 🆕️
  ]
}
```

#### 配置 主应用的 /\_\_dev

分别填写如下参数:

```shell
主应用名称: grafana
路由: /grafana
线上地址: -
调试地址: http://localhost:3000
指定菜单地址: http://localhost:3000/public/build/menu.json
启用调试地址: 设为开
```

注意设置上面的 `指定菜单地址`

如果 /\_\_dev 中不存在 `指定菜单地址` 列, 请更新环境镜像

## 非首次开发

首次开发完成后,依赖都安装完成, 之后可分别执行如下命令即可开始开发:

```shell
# 前端构建, 产物将放在 /public/build下, 文件有变化会自动build
yarn start
# 构建菜单
yarn build:menu
# 启动后端服务,同时将前端文件托管出去
make run
```
