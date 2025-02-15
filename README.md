# r2img
### 简述
以cloudflare r2作为后备存储,服务器本地作为临时存储,将png,jpg转webp,采用webp存储的个人图床
### 功能
后端api验证,png,jpg转webp,控制服务器本地存储总大小
### 使用方法
- /upload formdata file字段存放图片 响应返回文件名
- /i/:fileName 图片外链
### 优势
- 图片在r2有备份,不会丢失.
- 采用服务器本地临时存储,减少r2的请求次数
### 原理
- 上传: 向go web程序发起post formdata请求,检查api key是否正确,如果不正确返回401,里面含有字段名为file的文件,如果文件时png或者是jpg,转化成webp,如果是webp就不用转化,将webp转发到worker后端,worker后端将图片存入r2中.
- 读取: 向go web程序请求/i/:filename,检查本地是否有该文件,如果没有就向worker后端请求,worker检查r2是否有该文件,如果有就返回,如果没有,就返回404,go接受到图片会保存到本地,再发送响应.保存到本地会检查文件夹是否过大,如果过大,就会清除最旧的一部分文件.
### 部署
#### worker后端
新建cloudflare worker ,将`/worker/worker.js`里面的代码粘贴至剪贴板,配置变量.记住域名,如果go编译的二进制文件要在中国大陆运行,需要将worker绑定自定义域名,其他地区可以直接使用默认分配的域名.

|字段名|描述|
|-|-|
AUTH_API_KEY|worker api key 自定义高强度密码,推荐20位数字字母组合
R2_BUCKET|r2储存桶绑定
#### go后端
自行编译或者下载release的二进制文件,在相同目录新建`./i`文件夹,将`/go/config.example.json`的内容复制到相同路径`config.json`,修改配置文件信息

|字段名|描述|
|-|-|
api_site|worker域名,要到协议头
api_key|worker设置的api key
auth_key|go的api key上传文件需要
max_file_size|原文件最大大小
quality|png,jpeg转webp的质量,在0到100范围内,推荐80
port|go web程序开放端口
max_cache_size|本地`./i`缓存总大小限制,以MB为单位
free_cache_size|本地总大小超过max_cache_size后一次性删除后保留的大小,以MB为单位

启动脚本
### 后续开发计划
制作前端,优化删除缓存的代码
### 开发
`/worker`里面有源代码,自行部署或改进,`/go`目录是go的源码,自行编译改进