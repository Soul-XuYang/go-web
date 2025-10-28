## 配置信息

### docker配置数据库系统
系统为WSL2的Ubuntu20.04
这里先用docker pull mysql:8.0镜像然后再拉取其对应的容器，这里docker run --name my-mysql -p 13306:3306 
**实际的端口是13306,密码是123456，数据库名字是test**
```dockerfile
-e MYSQL_ROOT_PASSWORD=123456 
-e MYSQL_DATABASE=test   
-v /home/wxy/mysql-data:/var/lib/mysql  
-d mysql:8.0
```
建立链接:docker exec -it my-mysql mysql -uroot -p123456 -e "SHOW DATABASES;"
**注意**:docker start my-mysql启动容器，docker stop my-mysql停止容器，docker rm my-mysql删除容器,而首次创建 只需要 docker run 一次，数据已经持久化在我的文件目录下了

docker update --restart unless-stopped my-mysql  设置容器开机自启
