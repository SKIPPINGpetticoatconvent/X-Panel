FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies required by the script
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    sudo \
    tar \
    cron \
    procps \
    tzdata \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Mock systemctl to prevent script failure
RUN echo '#!/bin/bash\n\
if [[ "$1" == "status" ]]; then\n\
  # Mock running status for check_status\n\
  if [[ "$2" == "x-ui" ]]; then\n\
     echo "Active: active (running)"\n\
     exit 0\n\
  fi\n\
fi\n\
if [[ "$1" == "is-enabled" ]]; then\n\
  echo "enabled"\n\
  exit 0\n\
fi\n\
# Log other calls\n\
echo "systemctl called with args: $*" >> /tmp/systemctl.log\n\
exit 0' > /usr/bin/systemctl && chmod +x /usr/bin/systemctl

# Mock timedatectl
RUN echo '#!/bin/bash\nexit 0' > /usr/bin/timedatectl && chmod +x /usr/bin/timedatectl

# Mock service command
RUN echo '#!/bin/bash\nexit 0' > /usr/sbin/service && chmod +x /usr/sbin/service

WORKDIR /root
