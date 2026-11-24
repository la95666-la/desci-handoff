FROM hkccr.ccs.tencentyun.com/kol-base/box:v1.0.1
ARG MODULE
WORKDIR /home

RUN groupadd -g 1501 -r pm_user && useradd -u 1501 -r -g 1501 pm_user

COPY --chown=pm_user:pm_user ${MODULE}/config.yaml /home/config/${MODULE}/config.yaml
COPY --chown=pm_user:pm_user common/config.yaml /home/config/common/config.yaml
COPY --chown=pm_user:pm_user common/i18n /home/config/common/i18n
COPY --chown=pm_user:pm_user secret/config.yaml /home/config/secret/config.yaml
COPY --chown=pm_user:pm_user ${MODULE}/${MODULE} /home/${MODULE}
COPY --chown=pm_user:pm_user ${MODULE}/entrypoint /home/entrypoint

RUN chown -R pm_user:pm_user /home

USER pm_user

CMD ["bash", "./entrypoint"]
