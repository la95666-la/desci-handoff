FROM hkccr.ccs.tencentyun.com/kol-base/box:v1.0.1
WORKDIR /home

RUN groupadd -g 1501 -r pm_user && useradd -u 1501 -r -g 1501 pm_user

# 安装 nginx 并清除默认配置
RUN apt-get update && apt-get install -y nginx && rm -rf /var/lib/apt/lists/* \
    && rm -f /etc/nginx/sites-enabled/default \
    && rm -f /etc/nginx/conf.d/default.conf \
    && mkdir -p /var/lib/nginx/body /var/lib/nginx/proxy /var/lib/nginx/fastcgi /var/lib/nginx/uwsgi /var/lib/nginx/scgi

# 复制编译后的前端静态文件 (dist 目录)
COPY --chown=pm_user:pm_user dist /home/frontend/dist
# 复制 nginx 配置
COPY --chown=pm_user:pm_user nginx.conf /home/frontend/nginx.conf
# 复制启动脚本
COPY --chown=pm_user:pm_user entrypoint /home/entrypoint

RUN chown -R pm_user:pm_user /home && chmod +x /home/entrypoint

CMD ["bash", "/home/entrypoint"]
